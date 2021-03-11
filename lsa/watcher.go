package main

//#cgo CFLAGS: -x objective-c
//#cgo LDFLAGS: -framework CoreFoundation -framework CoreServices
//extern void watch(const char *path);
import "C"
import "unsafe"

var pathCh chan <- string

//export watchCallback
func watchCallback(s uintptr, info uintptr, n C.size_t, paths, flags, ids uintptr) {
	const offsetChar = unsafe.Sizeof((*C.char)(nil))

	for i := uintptr(0); i < uintptr(n); i++ {
		pathCh <- C.GoString(*(**C.char)(unsafe.Pointer(paths + i*offsetChar)))
	}
}

func Watch(path string, ch chan <- string) {
	pathCh = ch
	go C.watch(C.CString(path))
}
