module github.com/mrz1836/sigil

go 1.25.6

require (
	filippo.io/age v1.3.1
	github.com/agnivade/levenshtein v1.2.1
	github.com/ethereum/go-ethereum v1.16.8
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.11.1
	github.com/tyler-smith/go-bip32 v1.0.0
	github.com/tyler-smith/go-bip39 v1.1.0
	golang.org/x/crypto v0.47.0
	golang.org/x/sys v0.40.0
	golang.org/x/term v0.39.0
	golang.org/x/time v0.14.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	filippo.io/hpke v0.4.0 // indirect
	github.com/FactomProject/basen v0.0.0-20150613233007-fe3947df716e // indirect
	github.com/FactomProject/btcutilecc v0.0.0-20130527213604-d3a63a5752ec // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20260201044653-ee82dce4af02 // indirect
	github.com/bits-and-blooms/bitset v1.24.4 // indirect
	github.com/consensys/gnark-crypto v0.19.2 // indirect
	github.com/crate-crypto/go-eth-kzg v1.4.0 // indirect
	github.com/crate-crypto/go-ipa v0.0.0-20240724233137-53bbb0ceb27a // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/deckarep/golang-set/v2 v2.8.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0 // indirect
	github.com/ethereum/c-kzg-4844/v2 v2.1.5 // indirect
	github.com/ethereum/go-verkle v0.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/supranational/blst v0.3.16 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/sync v0.19.0 // indirect
)

replace (
	// CVE-2025-47908: rs/cors Resource Exhaustion - upgrade to fixed version
	github.com/rs/cors => github.com/rs/cors v1.11.0
	// CVE-2021-43668: goleveldb NULL Pointer Dereference - upgrade to latest commit (July 2022)
	github.com/syndtr/goleveldb => github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d
)
