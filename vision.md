# Sigil — Vision

## The Problem

Old wallets, lost keys, scattered funds across chains. BSV funds locked in old addresses. BTC dust sitting in legacy wallets. ETH tokens from 2017 airdrops.

## The Solution

A single CLI tool that:
1. Imports keys from various formats (WIF, mnemonic, keystore)
2. Scans for balances across chains
3. Generates transactions to consolidate/move funds
4. Signs and broadcasts with full control

## Success Looks Like

- All old BSV funds recovered and consolidated
- Clear inventory of all chain holdings
- One tool to rule them all

## Non-Goals

- Not a daily driver wallet
- Not for trading
- Not a web/mobile app
- No custodial features

## Open Questions

- Best Go libraries for each chain?
- Key derivation paths to support?
- UTXO scanning strategy for BSV?
