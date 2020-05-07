#include <CoreServices/CoreServices.h>
#include <CoreFoundation/CoreFoundation.h>

void watchCallback(const struct __FSEventStream *, void *, unsigned long, void *, const unsigned int *, const unsigned long long *);

void watch(const char *path) {
    CFStringRef pathToWatch = CFStringCreateWithCString(NULL, path, kCFStringEncodingUTF8);
    CFArrayRef pathsToWatch = CFArrayCreate(NULL, (const void **)&pathToWatch, 1, NULL);
    FSEventStreamRef stream = FSEventStreamCreate(
        NULL,
        &watchCallback,
        NULL,
        pathsToWatch,
        kFSEventStreamEventIdSinceNow,
        0.0,
        kFSEventStreamCreateFlagNone|kFSEventStreamCreateFlagNoDefer
    );

    CFRelease(pathsToWatch);
    CFRelease(pathToWatch);

    FSEventStreamScheduleWithRunLoop(stream, CFRunLoopGetCurrent(), kCFRunLoopDefaultMode);
    FSEventStreamStart(stream);
    CFRunLoopRun();
}
