//go:build !linux

package util

type JavaProgram struct{}

func AttachToJavaProgram(fileInfo *FileInfo) (*JavaProgram, error)                    { return nil, nil }
func (p *JavaProgram) DetachFromJavaProgram() error                                   { return nil }
func (p *JavaProgram) ReadMemoryIntoBuf(address uintptr, buf []byte, size int) error  { return nil }
func (p *JavaProgram) WriteBufInfoMemory(address uintptr, buf []byte, size int) error { return nil }
func (p *JavaProgram) ReadMemory(address uintptr, size int) ([]byte, error)           { return nil, nil }
func (p *JavaProgram) ReadUint64(address uintptr) (uint64, error)                     { return 0, nil }
func (p *JavaProgram) ReadSymbolValues(syms map[string]uintptr) (map[string]uint64, error) {
	return nil, nil
}
