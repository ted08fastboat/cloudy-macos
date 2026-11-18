//go:build darwin

package audio

import (
	"fmt"
	"sort"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// macOS's outputs are its CoreAudio devices, listed through the same
// framework the audio queue plays through: CoreAudio for the devices,
// CoreFoundation for their names and IDs, which it hands out as CFStrings.

const (
	kAudioObjectSystemObject = 1

	kAudioObjectPropertyScopeGlobal = 0x676c6f62 // 'glob'
	kAudioObjectPropertyScopeOutput = 0x6f757470 // 'outp'

	kAudioObjectPropertyElementMain = 0

	kAudioHardwarePropertyDevices                    = 0x64657623 // 'dev#'
	kAudioHardwarePropertyDefaultOutputDevice        = 0x644f7574 // 'dOut'
	kAudioDevicePropertyStreamConfiguration          = 0x736c6179 // 'slay'
	kAudioDevicePropertyDeviceUID                    = 0x75696420 // 'uid '
	kAudioObjectPropertyName                         = 0x6c6e616d // 'lnam'
	kCFStringEncodingUTF8                     uint32 = 0x08000100
)

// audioObjectPropertyAddress names a property of an object: which property,
// in which scope, of which element.
type audioObjectPropertyAddress struct {
	selector uint32
	scope    uint32
	element  uint32
}

// audioBuffer is one buffer of an AudioBufferList, which is its count
// followed by that many of them, the first on the 8 bytes a pointer wants.
type audioBuffer struct {
	channels uint32
	size     uint32
	data     unsafe.Pointer
}

type audioBufferList struct {
	count   uint32
	buffers [1]audioBuffer
}

var (
	coreAudioLoad sync.Once
	coreAudioErr  error

	objectGetPropertyData     func(obj uint32, addr *audioObjectPropertyAddress, qualSize uint32, qual unsafe.Pointer, size *uint32, data unsafe.Pointer) int32
	objectGetPropertyDataSize func(obj uint32, addr *audioObjectPropertyAddress, qualSize uint32, qual unsafe.Pointer, size *uint32) int32

	cfStringCreateWithCString func(alloc uintptr, s *byte, encoding uint32) uintptr
	cfStringGetCString        func(s uintptr, buf *byte, size int64, encoding uint32) uint8
	cfRelease                 func(cf uintptr)
)

func loadCoreAudio() error {
	coreAudioLoad.Do(func() {
		core, err := purego.Dlopen("/System/Library/Frameworks/CoreAudio.framework/CoreAudio", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			coreAudioErr = err
			return
		}
		foundation, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			coreAudioErr = err
			return
		}
		purego.RegisterLibFunc(&objectGetPropertyData, core, "AudioObjectGetPropertyData")
		purego.RegisterLibFunc(&objectGetPropertyDataSize, core, "AudioObjectGetPropertyDataSize")
		purego.RegisterLibFunc(&cfStringCreateWithCString, foundation, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&cfStringGetCString, foundation, "CFStringGetCString")
		purego.RegisterLibFunc(&cfRelease, foundation, "CFRelease")
	})
	return coreAudioErr
}

// listOutputs lists the devices sound can play through: those with a
// channel to render, the default first.
func listOutputs() ([]Output, error) {
	if err := loadCoreAudio(); err != nil {
		return nil, err
	}
	ids, err := devices()
	if err != nil {
		return nil, err
	}
	def, _ := defaultOutput()
	var outs []Output
	for _, id := range ids {
		if !plays(id) {
			continue
		}
		uid, err := stringProperty(id, kAudioDevicePropertyDeviceUID, kAudioObjectPropertyScopeGlobal)
		if err != nil || uid == "" {
			continue
		}
		name, err := stringProperty(id, kAudioObjectPropertyName, kAudioObjectPropertyScopeGlobal)
		if err != nil || name == "" {
			name = uid
		}
		outs = append(outs, Output{ID: uid, Name: name, Default: id == def})
	}
	sort.SliceStable(outs, func(i, j int) bool {
		if outs[i].Default != outs[j].Default {
			return outs[i].Default
		}
		return outs[i].Name < outs[j].Name
	})
	if len(outs) == 0 {
		return nil, errUnsupported
	}
	return outs, nil
}

// deviceOf returns the device of the output id, for the queue to play
// through, and whether it is listed now.
func deviceOf(id string) (uint32, bool) {
	ids, err := devices()
	if err != nil {
		return 0, false
	}
	for _, dev := range ids {
		if uid, err := stringProperty(dev, kAudioDevicePropertyDeviceUID, kAudioObjectPropertyScopeGlobal); err == nil && uid == id {
			return dev, true
		}
	}
	return 0, false
}

// devices are the audio devices the system has.
func devices() ([]uint32, error) {
	addr := audioObjectPropertyAddress{selector: kAudioHardwarePropertyDevices, scope: kAudioObjectPropertyScopeGlobal, element: kAudioObjectPropertyElementMain}
	var size uint32
	if st := objectGetPropertyDataSize(kAudioObjectSystemObject, &addr, 0, nil, &size); st != 0 {
		return nil, fmt.Errorf("audio: AudioObjectGetPropertyDataSize failed (OSStatus %d)", st)
	}
	if size == 0 {
		return nil, nil
	}
	ids := make([]uint32, size/4)
	if st := objectGetPropertyData(kAudioObjectSystemObject, &addr, 0, nil, &size, unsafe.Pointer(&ids[0])); st != 0 {
		return nil, fmt.Errorf("audio: AudioObjectGetPropertyData failed (OSStatus %d)", st)
	}
	return ids[:size/4], nil
}

// defaultOutput is the device the system plays through unless told otherwise.
func defaultOutput() (uint32, error) {
	addr := audioObjectPropertyAddress{selector: kAudioHardwarePropertyDefaultOutputDevice, scope: kAudioObjectPropertyScopeGlobal, element: kAudioObjectPropertyElementMain}
	var id uint32
	size := uint32(4)
	if st := objectGetPropertyData(kAudioObjectSystemObject, &addr, 0, nil, &size, unsafe.Pointer(&id)); st != 0 {
		return 0, fmt.Errorf("audio: no default output (OSStatus %d)", st)
	}
	return id, nil
}

// plays reports whether the device renders sound: it has a channel on its
// output side, where a microphone has none.
func plays(id uint32) bool {
	addr := audioObjectPropertyAddress{selector: kAudioDevicePropertyStreamConfiguration, scope: kAudioObjectPropertyScopeOutput, element: kAudioObjectPropertyElementMain}
	var size uint32
	if st := objectGetPropertyDataSize(id, &addr, 0, nil, &size); st != 0 || size < 4 {
		return false
	}
	// The buffers' count, then the buffers: their channels say whether
	// there is one to play through.
	buf := make([]byte, max(int(size), int(unsafe.Sizeof(audioBufferList{}))))
	if st := objectGetPropertyData(id, &addr, 0, nil, &size, unsafe.Pointer(&buf[0])); st != 0 {
		return false
	}
	list := (*audioBufferList)(unsafe.Pointer(&buf[0]))
	if list.count == 0 || int(size) < int(unsafe.Sizeof(audioBufferList{})) {
		return false
	}
	channels := uint32(0)
	for _, b := range unsafe.Slice(&list.buffers[0], list.count) {
		channels += b.channels
	}
	return channels > 0
}

// stringProperty reads a CFString property of the object, as a Go string.
func stringProperty(obj uint32, selector, scope uint32) (string, error) {
	addr := audioObjectPropertyAddress{selector: selector, scope: scope, element: kAudioObjectPropertyElementMain}
	var ref uintptr
	size := uint32(unsafe.Sizeof(ref))
	if st := objectGetPropertyData(obj, &addr, 0, nil, &size, unsafe.Pointer(&ref)); st != 0 || ref == 0 {
		return "", fmt.Errorf("audio: string property %#x failed (OSStatus %d)", selector, st)
	}
	defer cfRelease(ref)
	buf := make([]byte, 512)
	if cfStringGetCString(ref, &buf[0], int64(len(buf)), kCFStringEncodingUTF8) == 0 {
		return "", fmt.Errorf("audio: string property %#x is not UTF-8", selector)
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i]), nil
		}
	}
	return string(buf), nil
}

// cfString makes a CFString of s, for the calls that take one, to be
// released with cfRelease.
func cfString(s string) uintptr {
	c := append([]byte(s), 0)
	return cfStringCreateWithCString(0, &c[0], kCFStringEncodingUTF8)
}
