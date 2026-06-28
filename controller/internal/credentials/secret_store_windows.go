//go:build windows

package credentials

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32                = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

func saveSecret(key, value string) error {
	path, err := secretFilePath(key)
	if err != nil {
		return err
	}
	encrypted, err := protectData([]byte(value))
	if err != nil {
		return err
	}
	return os.WriteFile(path, encrypted, 0600)
}

func loadSecret(key string) (string, error) {
	path, err := secretFilePath(key)
	if err != nil {
		return "", err
	}
	encrypted, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	plain, err := unprotectData(encrypted)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func deleteSecret(key string) error {
	path, err := secretFilePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func protectData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		data = []byte{0}
	}
	in := dataBlob{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}
	var out dataBlob
	ret, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	result := make([]byte, out.cbData)
	copy(result, unsafe.Slice(out.pbData, out.cbData))
	return result, nil
}

func unprotectData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errSecretNotFound
	}
	in := dataBlob{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}
	var out dataBlob
	ret, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	result := make([]byte, out.cbData)
	copy(result, unsafe.Slice(out.pbData, out.cbData))
	if len(result) == 1 && result[0] == 0 {
		return nil, nil
	}
	return result, nil
}
