# Adaptive Codec for Organic Molecular Archives (ADS Codex)

ADS Codex is a DNA storage codec that provides high density and can adapt
to different requirements for DNA synthesis and sequencing.

## External Dependencies

### Reed-Solomon Package

ADS Codex depends on https://github.com/klauspost/reedsolomon

Please install it using 

```bash
go get -u github.com/klauspost/reedsolomon
```

### Lookup Tables

Lookup tables speed up significantly ADS Codex. You can generate them
using the tblgen tool (see below), or download them from github (1.7
GB file):

https://github.com/lanl/acoma/releases/download/0.9/tables.zip

Unpack the zip into the tbl directory where the tools and the unit
tests are expectin the lookup tables.

## Installation

To get ADS Codex clone this repository and build the packages and commands
that you are interested in.

## Documentation

The specification of the codec is located in the slides located in the
doc directory. More documentation on the implementation is located in
the source code.

## Packages

### oligo

Contains the basic abstraction of an oligo that is used by the rest of
the packages.

### oligo/short

An implementation of the basic oligo interface that stores an oligo in
a 64-bit integer, and therefore can handle short oligos (up to 32 nts).

### oligo/long

An implementation of the basic oligo interface that can store an
arbitrary long oligo. It uses one byte per nt.

### criteria

Abstract interface for oligo viability criteria. It is used by the
Level 0 codec (l0) to check if an oligo can be synthesized/sequenced.
The package implements a single criteria: H4G2 that prevents oligos
with homopolymers longer than 4 nts (for A, T, and C) or 2 nts for G.

### l0

Level 0 of the ADS Codex codec (bit packing). Theoretically it can pack
any value up to 64 bits. In practice it is prohibitively slow to pack
large values and requires lookup tables even for 17 bit values to
achieve reasonable performance.

### l1

Level 1 of the ADS Codex codec. Packs an address and array of bytes into a
single oligo.

### l2

Level 2 of the ADS Codex codec. Packs an arbitrary array of bytes into a
collection of oligos. Provides erasure code oligos for recove of the
data in case of errors.

## Tools

The tools in the repository use the packages to provide some
convenient commands.

### tblgen

Generates encoding and decoding lookup tables for speeding-up the
Level 0 encoding and decoding. 

For example, generating an encoding lookup table for 17 nts oligos
that has 2^13 entries can be done by:

```bash
./tblgen -e encnt17b13.tbl -l 17 -b 13
```

Generating a decoding lookup table for 17 nts oligos that has 2^14
entries can be done by:

```bash
./tblgen -d decnt17b7.tbl -l 17 -b 7
```

Although the code is parallelized and uses all available cores, it can
take few hours to generate the table.

### encode

Encodes the specified file and outputs a list of oligos that represent
it.

### decode

Decodes the specified list of oligos into a file. If not all data can
be recovered, the output file might have holes.

## Unit Tests

The packages have some limited unit tests that can be run by the
standard:

```bash
go test
```

The unit tests will slowly be extended to cover all use cases.

## Limitations

There are multiple TODO and FIXME comments in the source code that
describe things that are missing, or implementation restrictions that
should be fixed eventually.
