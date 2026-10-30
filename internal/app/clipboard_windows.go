package app

import (
	"errors"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                      = syscall.NewLazyDLL("user32.dll")
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	shell32                     = syscall.NewLazyDLL("shell32.dll")
	procOpenClipboard           = user32.NewProc("OpenClipboard")
	procCloseClipboard          = user32.NewProc("CloseClipboard")
	procEmptyClipboard          = user32.NewProc("EmptyClipboard")
	procSetClipboardData        = user32.NewProc("SetClipboardData")
	procGetClipboardData        = user32.NewProc("GetClipboardData")
	procDragQueryFile           = shell32.NewProc("DragQueryFileW")
	procRegisterClipboardFormat = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc             = kernel32.NewProc("GlobalAlloc")
	procGlobalFree              = kernel32.NewProc("GlobalFree")
	procGlobalLock              = kernel32.NewProc("GlobalLock")
	procGlobalUnlock            = kernel32.NewProc("GlobalUnlock")
	procMoveMemory              = kernel32.NewProc("RtlMoveMemory")
)

const (
	clipboardUnicodeText = 13
	clipboardFileDrop    = 15 // CF_HDROP
	globalMoveable       = 0x0002
	dropEffectCopy       = 1
	// Size of DROPFILES: pFiles, pt (x, y), fNC, fWide.
	dropFilesHeader = 20
)

// setClipboardFiles puts files on the system clipboard the way Explorer
// copies them (CF_HDROP, marked as a copy), so a paste in Explorer or
// another program copies the files. text goes along as plain text.
func setClipboardFiles(paths []string, text string) error {
	var names []uint16
	for _, path := range paths {
		name, err := syscall.UTF16FromString(path)
		if err != nil {
			return err
		}
		names = append(names, name...)
	}
	names = append(names, 0)
	drop := make([]byte, dropFilesHeader+len(names)*2)
	drop[0] = dropFilesHeader // pFiles: the names follow the header
	drop[16] = 1              // fWide: the names are UTF-16
	copy(drop[dropFilesHeader:], unsafe.Slice((*byte)(unsafe.Pointer(&names[0])), len(names)*2))

	textUTF16, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}
	effectName, _ := syscall.UTF16PtrFromString("Preferred DropEffect")
	effectFormat, _, _ := procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(effectName)))

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()
	if ok, _, err := procEmptyClipboard.Call(); ok == 0 {
		return err
	}
	if err := setClipboardData(clipboardFileDrop, drop); err != nil {
		return err
	}
	if effectFormat != 0 {
		if err := setClipboardData(effectFormat, []byte{dropEffectCopy, 0, 0, 0}); err != nil {
			return err
		}
	}
	return setClipboardData(clipboardUnicodeText, unsafe.Slice((*byte)(unsafe.Pointer(&textUTF16[0])), len(textUTF16)*2))
}

// clipboardFiles returns the paths of files on the system clipboard (copied
// in Explorer); none when it holds no files.
func clipboardFiles() ([]string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := openClipboard(); err != nil {
		return nil, err
	}
	defer procCloseClipboard.Call()
	drop, _, _ := procGetClipboardData.Call(clipboardFileDrop)
	if drop == 0 {
		return nil, nil
	}
	count, _, _ := procDragQueryFile.Call(drop, 0xFFFFFFFF, 0, 0)
	paths := make([]string, 0, count)
	for i := range count {
		length, _, _ := procDragQueryFile.Call(drop, i, 0, 0)
		name := make([]uint16, length+1)
		procDragQueryFile.Call(drop, i, uintptr(unsafe.Pointer(&name[0])), length+1)
		paths = append(paths, syscall.UTF16ToString(name))
	}
	return paths, nil
}

// openClipboard retries for a moment: another program may hold the
// clipboard open.
func openClipboard() error {
	var err error
	for range 10 {
		var ok uintptr
		if ok, _, err = procOpenClipboard.Call(0); ok != 0 {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return errors.Join(errors.New("the clipboard is in use"), err)
}

// setClipboardData copies data into global memory the clipboard then owns.
func setClipboardData(format uintptr, data []byte) error {
	handle, _, err := procGlobalAlloc.Call(globalMoveable, uintptr(len(data)))
	if handle == 0 {
		return err
	}
	memory, _, err := procGlobalLock.Call(handle)
	if memory == 0 {
		procGlobalFree.Call(handle)
		return err
	}
	procMoveMemory.Call(memory, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	procGlobalUnlock.Call(handle)
	if ok, _, err := procSetClipboardData.Call(format, handle); ok == 0 {
		procGlobalFree.Call(handle)
		return err
	}
	return nil
}
