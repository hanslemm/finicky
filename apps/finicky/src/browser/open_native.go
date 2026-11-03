package browser

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

// Result codes for finickyOpen.
enum {
	FINICKY_OPEN_OK = 0,
	FINICKY_OPEN_UNRESOLVED = 1, // app or URL not resolvable here: caller falls back to /usr/bin/open
	FINICKY_OPEN_FAILED = 2,     // LaunchServices reported an error (message in *errOut)
	FINICKY_OPEN_TIMEOUT = 3,    // no completion within the wait budget
};

#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
static NSURL *finickyResolveApp(NSString *app, int isBundleID) {
	NSWorkspace *ws = [NSWorkspace sharedWorkspace];
	if (isBundleID) {
		return [ws URLForApplicationWithBundleIdentifier:app];
	}
	if ([app hasPrefix:@"/"]) {
		return [NSURL fileURLWithPath:app];
	}
	// Same lookup `open -a <name>` does. Deprecated, but still the only API
	// that resolves an app by display name.
	NSString *path = [ws fullPathForApplication:app];
	return path ? [NSURL fileURLWithPath:path] : nil;
}
#pragma clang diagnostic pop

// finickyOpen opens url in the given app, or launches the app with args when
// passURL is 0. It waits for LaunchServices to finish so errors are reported
// and the process does not exit while the request is in flight.
static int finickyOpen(const char *app, int isBundleID, const char *url, int background,
                       int newInstance, const char **args, int nargs, int passURL, char **errOut) {
	@autoreleasepool {
		NSURL *appURL = finickyResolveApp([NSString stringWithUTF8String:app], isBundleID);
		if (!appURL) return FINICKY_OPEN_UNRESOLVED;

		NSURL *target = nil;
		if (passURL) {
			target = [NSURL URLWithString:[NSString stringWithUTF8String:url]];
			if (!target) return FINICKY_OPEN_UNRESOLVED;
		}

		NSWorkspaceOpenConfiguration *cfg = [NSWorkspaceOpenConfiguration configuration];
		cfg.activates = background ? NO : YES;
		cfg.createsNewApplicationInstance = newInstance ? YES : NO;
		if (nargs > 0) {
			NSMutableArray<NSString *> *list = [NSMutableArray arrayWithCapacity:nargs];
			for (int i = 0; i < nargs; i++) {
				[list addObject:[NSString stringWithUTF8String:args[i]]];
			}
			cfg.arguments = list;
		}

		// The semaphore and the error string outlive this call when the wait
		// times out, so the block owns them (manual retain/release: no ARC).
		dispatch_semaphore_t done = dispatch_semaphore_create(0);
		__block char *errCopy = NULL;
		void (^completion)(NSRunningApplication *, NSError *) = ^(NSRunningApplication *a, NSError *e) {
			if (e) errCopy = strdup([[e localizedDescription] UTF8String]);
			dispatch_semaphore_signal(done);
		};

		NSWorkspace *ws = [NSWorkspace sharedWorkspace];
		if (passURL) {
			[ws openURLs:@[target] withApplicationAtURL:appURL configuration:cfg completionHandler:completion];
		} else {
			[ws openApplicationAtURL:appURL configuration:cfg completionHandler:completion];
		}

		if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC)) != 0) {
			// Leak the semaphore on purpose: the completion block may still signal it.
			return FINICKY_OPEN_TIMEOUT;
		}
		dispatch_release(done);
		if (errCopy) {
			*errOut = errCopy;
			return FINICKY_OPEN_FAILED;
		}
		return FINICKY_OPEN_OK;
	}
}
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"
)

// errNativeUnresolved means the native path cannot handle this request; the
// caller falls back to /usr/bin/open, which has its own lookup rules.
var errNativeUnresolved = errors.New("native open: app or URL not resolvable")

// openNative asks LaunchServices directly, through NSWorkspace. This avoids
// starting /usr/bin/open as a child process for each URL.
//
// When passURL is false the app is launched with args only (the URL, if any,
// is one of the args) — the same as `open -a App --args ...`.
func openNative(app string, isBundleID bool, url string, background, newInstance bool, args []string, passURL bool) error {
	// C strings end at NUL. A value with a NUL would be cut short on the C
	// side, so leave it to /usr/bin/open, which refuses it.
	if strings.IndexByte(app, 0) >= 0 || strings.IndexByte(url, 0) >= 0 {
		return errNativeUnresolved
	}
	for _, a := range args {
		if strings.IndexByte(a, 0) >= 0 {
			return errNativeUnresolved
		}
	}
	cApp := C.CString(app)
	defer C.free(unsafe.Pointer(cApp))
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))

	var cArgs **C.char
	if len(args) > 0 {
		cArgs = (**C.char)(C.malloc(C.size_t(len(args)) * C.size_t(unsafe.Sizeof(uintptr(0)))))
		defer C.free(unsafe.Pointer(cArgs))
		list := unsafe.Slice(cArgs, len(args))
		for i, a := range args {
			list[i] = C.CString(a)
			defer C.free(unsafe.Pointer(list[i]))
		}
	}

	var cErr *C.char
	rc := C.finickyOpen(cApp, cBool(isBundleID), cURL, cBool(background), cBool(newInstance),
		cArgs, C.int(len(args)), cBool(passURL), &cErr)

	switch rc {
	case C.FINICKY_OPEN_OK:
		return nil
	case C.FINICKY_OPEN_UNRESOLVED:
		return errNativeUnresolved
	case C.FINICKY_OPEN_TIMEOUT:
		return fmt.Errorf("timed out waiting for %s to open", app)
	default:
		msg := "unknown error"
		if cErr != nil {
			msg = C.GoString(cErr)
			C.free(unsafe.Pointer(cErr))
		}
		return fmt.Errorf("failed to open %s: %s", app, msg)
	}
}

func cBool(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
