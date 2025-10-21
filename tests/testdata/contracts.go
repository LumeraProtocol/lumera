package testdata

import (
	_ "embed"

)

var (
	//go:embed reflect_2_0.wasm
	reflectContract []byte
	//go:embed ibc_reflect.wasm
	ibcReflectContract []byte
	//go:embed burner.wasm
	burnerContract []byte
	//go:embed hackatom.wasm
	hackatomContract []byte
)

func ReflectContractWasm() []byte {
	return reflectContract
}

func IBCReflectContractWasm() []byte {
	return ibcReflectContract
}

func BurnerContractWasm() []byte {
	return burnerContract
}

func HackatomContractWasm() []byte {
	return hackatomContract
}
