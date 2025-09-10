//go:build !linux

package util

func GetTmpPath(pid int) string                                    { return "" }
func GetProcessInfo(pid int, uid *int, gid *int, nspid *int) error { return nil }
func EnterNS(pid int, nsType string) int                           { return 0 }
