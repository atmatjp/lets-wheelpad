package device

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/holoplot/go-evdev"
)

const (
	uinputMaxNameSize = 80
	absSize           = 64
)

type uinputUserDev struct {
	Name       [uinputMaxNameSize]byte
	ID         evdev.InputID
	EffectsMax uint32
	Absmax     [absSize]int32
	Absmin     [absSize]int32
	Absfuzz    [absSize]int32
	Absflat    [absSize]int32
}

func ioctl(fd uintptr, code uint32, ptr unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(code), uintptr(ptr))
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlMakeCode(dir, typ, nr int, size uintptr) uint32 {
	return uint32(dir)<<30 | uint32(size)<<16 | uint32(typ)<<8 | uint32(nr)
}

func uiSetEvBit() uint32   { return ioctlMakeCode(0x1, 'U', 100, unsafe.Sizeof(int32(0))) }
func uiSetKeyBit() uint32  { return ioctlMakeCode(0x1, 'U', 101, unsafe.Sizeof(int32(0))) }
func uiSetRelBit() uint32  { return ioctlMakeCode(0x1, 'U', 102, unsafe.Sizeof(int32(0))) }
func uiSetAbsBit() uint32  { return ioctlMakeCode(0x1, 'U', 103, unsafe.Sizeof(int32(0))) }
func uiSetPropBit() uint32 { return ioctlMakeCode(0x1, 'U', 110, unsafe.Sizeof(int32(0))) }
func uiDevCreate() uint32  { return ioctlMakeCode(0, 'U', 1, 0) }

type VirtualTouchpad struct {
	file *os.File
}

func CreateVirtualTouchpad(source *evdev.InputDevice) (*VirtualTouchpad, error) {
	fd, err := os.OpenFile("/dev/uinput", os.O_WRONLY|syscall.O_NONBLOCK, 0660)
	if err != nil {
		return nil, err
	}

	for _, t := range source.CapableTypes() {
		ioctl(fd.Fd(), uiSetEvBit(), unsafe.Pointer(uintptr(t)))
		for _, c := range source.CapableEvents(t) {
			var code uint32
			switch t {
			case evdev.EV_KEY: code = uiSetKeyBit()
			case evdev.EV_REL: code = uiSetRelBit()
			case evdev.EV_ABS: code = uiSetAbsBit()
			default: continue
			}
			ioctl(fd.Fd(), code, unsafe.Pointer(uintptr(c)))
		}
	}

	for _, p := range source.Properties() {
		ioctl(fd.Fd(), uiSetPropBit(), unsafe.Pointer(uintptr(p)))
	}

	uudev := uinputUserDev{}
	copy(uudev.Name[:], []byte("LetsNote-Virtual-Pad"))
	id, _ := source.InputID()
	uudev.ID = id

	absInfos, _ := source.AbsInfos()
	for code, info := range absInfos {
		if code < absSize {
			uudev.Absmax[code] = info.Maximum
			uudev.Absmin[code] = info.Minimum
			uudev.Absfuzz[code] = info.Fuzz
			uudev.Absflat[code] = info.Flat
		}
	}

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uudev)
	fd.Write(buf.Bytes())
	ioctl(fd.Fd(), uiDevCreate(), nil)

	return &VirtualTouchpad{file: fd}, nil
}

func (v *VirtualTouchpad) Emit(e *evdev.InputEvent) {
	var raw struct {
		Time  syscall.Timeval
		Type  uint16
		Code  uint16
		Value int32
	}
	raw.Time = e.Time
	raw.Type = uint16(e.Type)
	raw.Code = uint16(e.Code)
	raw.Value = e.Value

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, raw)
	v.file.Write(buf.Bytes())
}

func (v *VirtualTouchpad) Close() {
	v.file.Close()
}

func FindDevice(name string) (*evdev.InputDevice, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, err
	}

	for _, p := range paths {
		if p.Name == name {
			return evdev.Open(p.Path)
		}
	}

	return nil, fmt.Errorf("device not found: %s", name)
}
