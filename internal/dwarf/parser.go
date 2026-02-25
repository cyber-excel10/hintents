// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package dwarf provides DWARF debug information parsing for Soroban contract debugging.
// It extracts local variable information from WASM files with debug symbols to help
// reconstruct variable values at the point of a trap (e.g., memory-out-of-bounds).
package dwarf

import (
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	// ErrNoDebugInfo indicates the binary doesn't contain DWARF debug information
	ErrNoDebugInfo = errors.New("no DWARF debug information found")
	// ErrNoLocalVars indicates no local variables were found at the given address
	ErrNoLocalVars = errors.New("no local variables found at address")
	// ErrInvalidWASM indicates the file is not a valid WASM or ELF binary
	ErrInvalidWASM = errors.New("invalid WASM or ELF binary")
)

// LocalVar represents a local variable at a specific program location
type LocalVar struct {
	Name         string      // Variable name (may be mangled)
	DemangledName string    // Demangled name for display
	Type         string      // Type name
	Location     string      // DWARF location description
	Value        interface{} // Computed value (if available)
	Address      uint64      // Memory address (if applicable)
	StartLine    int         // Source line where variable is in scope
	EndLine      int         // Source line where variable goes out of scope
}

// SubprogramInfo represents a function/subprogram's debug information
type SubprogramInfo struct {
	Name           string
	DemangledName  string
	LowPC          uint64
	HighPC         uint64
	Line           int
	File           string
	LocalVariables []LocalVar
}

// SourceLocation represents a location in the source code
type SourceLocation struct {
	File   string
	Line   int
	Column int
}

// Frame represents a stack frame with local variable information
type Frame struct {
	Function     string
	SourceLoc    SourceLocation
	LocalVars    []LocalVar
	ReturnAddr   uint64
	FramePointer uint64
}

// Parser handles DWARF debug information extraction
type Parser struct {
	data      *dwarf.Data
	unit      *dwarf.Unit
	reader    *dwarf.Reader
	binaryType string // "wasm", "elf", "macho", "pe"
}

// NewParser creates a new DWARF parser from a binary
func NewParser(data []byte) (*Parser, error) {
	if len(data) < 4 {
		return nil, ErrInvalidWASM
	}

	// Detect binary type and try to parse DWARF info
	// Check for WASM (WebAssembly) magic number
	if data[0] == 0x00 && data[1] == 0x61 && data[2] == 0x73 && data[3] == 0x6d {
		return parseWASM(data)
	}

	// Try ELF
	if data[0] == 0x7f && data[1] == 0x45 && data[2] == 0x4c && data[3] == 0x46 {
		return parseELF(data)
	}

	// Try Mach-O
	if len(data) >= 4 {
		if binary.BigEndian.Uint32(data[0:4]) == 0xfeedfacf ||
			binary.LittleEndian.Uint32(data[0:4]) == 0xfeedfacf {
			return parseMacho(data)
		}
	}

	// Try PE
	if len(data) >= 2 {
		if binary.LittleEndian.Uint16(data[0:2]) == 0x5a4d {
			return parsePE(data)
		}
	}

	return nil, ErrInvalidWASM
}

// parseWASM parses DWARF info from a WASM binary
func parseWASM(data []byte) (*Parser, error) {
	// For WASM, we need to look for custom sections starting with ".debug_"
	// WASM doesn't have native DWARF support, but compilers often embed
	// DWARF info as custom sections
	
	// Try to find debug sections in WASM
	sections := parseWASMSections(data)
	
	var dwarfData *dwarf.Data
	var err error
	
	// Look for .debug_info section
	if infoSection, ok := sections[".debug_info"]; ok {
		dwarfData, err = dwarf.New(infoSection, nil, nil)
		if err != nil {
			// Try with line section too
			if lineSection, ok := sections[".debug_line"]; ok {
				dwarfData, err = dwarf.New(infoSection, lineSection, nil)
			}
		}
	}

	if dwarfData == nil || err != nil {
		// No DWARF info in WASM
		return nil, ErrNoDebugInfo
	}

	return &Parser{
		data:       dwarfData,
		binaryType: "wasm",
	}, nil
}

// parseWASMSections parses custom sections from a WASM binary
func parseWASMSections(data []byte) map[string][]byte {
	sections := make(map[string][]byte)
	
	i := 8 // Skip WASM magic + version
	for i < len(data) {
		if i+1 >= len(data) {
			break
		}
		sectionID := data[i]
		i++
		
		// Read section size (varint)
		sectionStart := i
		sectionSize := 0
		shift := 0
		for sectionStart+i-sectionStart < len(data) {
			b := data[sectionStart+i-sectionStart]
			sectionSize |= int(b&0x7f) << shift
			i++
			if b&0x80 == 0 {
				break
			}
			shift += 7
		}
		
		if sectionID == 0 { // Custom section
			nameStart := i
			nameLen := 0
			shift = 0
			for nameStart+nameLen-nameStart < len(data) {
				b := data[nameStart+nameLen-nameStart]
				nameLen |= int(b&0x7f) << shift
				nameLen++
				if b&0x80 == 0 {
					break
				}
				shift += 7
			}
			
			if nameStart+nameLen-nameStart < len(data) {
				name := string(data[nameStart:nameLen])
				contentStart := nameStart + nameLen
				contentEnd := contentStart + sectionSize - (nameLen - nameStart)
				if contentEnd <= len(data) {
					sections[name] = data[contentStart:contentEnd]
				}
			}
		}
		
		i += sectionSize
	}
	
	return sections
}

// parseELF parses DWARF info from an ELF binary
func parseELF(data []byte) (*Parser, error) {
	// Create a temporary file to use debug/elf package
	elfFile, err := elf.NewFile(bytesToReader(data))
	if err != nil {
		return nil, err
	}

	dwarfData, err := elfFile.DWARF()
	if err != nil {
		return nil, ErrNoDebugInfo
	}

	return &Parser{
		data:       dwarfData,
		binaryType: "elf",
	}, nil
}

// parseMacho parses DWARF info from a Mach-O binary
func parseMacho(data []byte) (*Parser, error) {
	machoFile, err := macho.NewFile(bytesToReader(data))
	if err != nil {
		return nil, err
	}

	dwarfData, err := machoFile.DWARF()
	if err != nil {
		return nil, ErrNoDebugInfo
	}

	return &Parser{
		data:       dwarfData,
		binaryType: "macho",
	}, nil
}

// parsePE parses DWARF info from a PE binary
func parsePE(data []byte) (*Parser, error) {
	peFile, err := pe.NewFile(bytesToReader(data))
	if err != nil {
		return nil, err
	}

	dwarfData, err := peFile.DWARF()
	if err != nil {
		return nil, ErrNoDebugInfo
	}

	return &Parser{
		data:       dwarfData,
		binaryType: "pe",
	}, nil
}

// bytesToReader converts a byte slice to an io.ReaderAt
type bytesReader struct {
	data []byte
	off  int
}

func (r *bytesReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n = copy(p, r.data[off:])
	return n, nil
}

func bytesToReader(data []byte) io.ReaderAt {
	return &bytesReader{data: data}
}

// GetSubprograms returns all subprograms (functions) in the debug info
func (p *Parser) GetSubprograms() ([]SubprogramInfo, error) {
	if p.data == nil {
		return nil, ErrNoDebugInfo
	}

	var subprograms []SubprogramInfo

	reader := p.data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil {
			break
		}
		if entry == nil {
			break
		}

		if entry.Tag == dwarf.TagSubprogram {
			subprogram, err := p.extractSubprogram(entry)
			if err == nil {
				subprograms = append(subprograms, subprogram)
			}
		}
	}

	return subprograms, nil
}

// extractSubprogram extracts information about a function/subprogram
func (p *Parser) extractSubprogram(entry *dwarf.Entry) (SubprogramInfo, error) {
	info := SubprogramInfo{}

	// Extract name
	if name, ok := entry.Val(dwarf.AttrName).(string); ok {
		info.Name = name
	}

	// Extract demangled name (if available)
	if demangled, ok := entry.Val(dwarf.AttrLinkageName).(string); ok {
		info.DemangledName = demangled
	} else {
		info.DemangledName = nameDemangle(info.Name)
	}

	// Extract low PC
	if lowPC, ok := entry.Val(dwarf.AttrLowpc).(uint64); ok {
		info.LowPC = lowPC
	}

	// Extract high PC
	if highPC, ok := entry.Val(dwarf.AttrHighpc).(uint64); ok {
		info.HighPC = highPC
	}

	// Extract line number
	if line, ok := entry.Val(dwarf.AttrDeclLine).(int64); ok {
		info.Line = int(line)
	}

	// Extract file
	if file, ok := entry.Val(dwarf.AttrDeclFile).(string); ok {
		info.File = file
	}

	// Get local variables for this subprogram
	info.LocalVariables = p.getLocalVariables(entry)

	return info, nil
}

// getLocalVariables extracts local variables for a subprogram
func (p *Parser) getLocalVariables(subprog *dwarf.Entry) []LocalVar {
	var locals []LocalVar

	reader := p.data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil || entry == nil {
			break
		}

		// Look for variables that are children of this subprogram
		if entry.Tag == dwarf.TagVariable || entry.Tag == dwarf.TagFormalParameter {
			// Check if this variable belongs to our subprogram
			if ref, ok := entry.Val(dwarf.AttrParent).(dwarf.Offset); ok && ref == subprog.Offset {
				local := p.extractLocalVar(entry)
				if local.Name != "" {
					locals = append(locals, local)
				}
			}
		}

		// Stop when we exit the current compilation unit
		if entry.Tag == 0 {
			break
		}
	}

	return locals
}

// extractLocalVar extracts information about a local variable
func (p *Parser) extractLocalVar(entry *dwarf.Entry) LocalVar {
	local := LocalVar{}

	// Get variable name
	if name, ok := entry.Val(dwarf.AttrName).(string); ok {
		local.Name = name
		local.DemangledName = nameDemangle(name)
	}

	// Get type
	if typ, ok := entry.Val(dwarf.AttrType).(dwarf.Offset); ok {
		local.Type = p.getTypeName(typ)
	}

	// Get location
	if loc, ok := entry.Val(dwarf.AttrLocation).([]byte); ok {
		local.Location = formatLocation(loc)
	}

	// Get line number
	if line, ok := entry.Val(dwarf.AttrDeclLine).(int64); ok {
		local.StartLine = int(line)
		local.EndLine = int(line)
	}

	return local
}

// getTypeName returns the name of a type given its offset
func (p *Parser) getTypeName(typeOffset dwarf.Offset) string {
	reader := p.data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil || entry == nil {
			break
		}

		if entry.Offset == typeOffset {
			switch entry.Tag {
			case dwarf.TagTypedef:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return name
				}
			case dwarf.TagBaseType:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return name
				}
			case dwarf.TagStructType:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return name
				}
			case dwarf.TagUnionType:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return name
				}
			case dwarf.TagEnumerationType:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return name
				}
			case dwarf.TagPointerType:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return "*" + name
				}
			}
		}

		if entry.Tag == 0 {
			break
		}
	}

	return "unknown"
}

// FindSubprogramAt finds the subprogram containing the given address
func (p *Parser) FindSubprogramAt(addr uint64) (*SubprogramInfo, error) {
	subprograms, err := p.GetSubprograms()
	if err != nil {
		return nil, err
	}

	for i := range subprograms {
		s := &subprograms[i]
		if addr >= s.LowPC && addr < s.HighPC {
			return s, nil
		}
	}

	return nil, fmt.Errorf("no subprogram found at address 0x%x", addr)
}

// FindLocalVarsAt finds local variables visible at the given address
func (p *Parser) FindLocalVarsAt(addr uint64) ([]LocalVar, error) {
	subprogram, err := p.FindSubprogramAt(addr)
	if err != nil {
		return nil, err
	}

	// Filter variables that are in scope at this address
	var inScope []LocalVar
	for _, v := range subprogram.LocalVariables {
		if addr >= uint64(v.StartLine) { // Simplified check
			inScope = append(inScope, v)
		}
	}

	if len(inScope) == 0 {
		return nil, ErrNoLocalVars
	}

	return inScope, nil
}

// GetSourceLocation finds the source location for a given address
func (p *Parser) GetSourceLocation(addr uint64) (*SourceLocation, error) {
	if p.data == nil {
		return nil, ErrNoDebugInfo
	}

	// Use the line information from DWARF
	reader := p.data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil || entry == nil {
			break
		}

		if entry.Tag == dwarf.TagCompileUnit {
			// Get line program for this unit
			if lineOffset, ok := entry.Val(dwarf.AttrStmtList).(uint64); ok {
				lp, err := p.data.LineProgram(entry.Offset, true)
				if err == nil {
					loc := p.findLineInProgram(lp, addr)
					if loc != nil {
						return loc, nil
					}
				}
			}
		}

		if entry.Tag == 0 {
			break
		}
	}

	return nil, fmt.Errorf("no source location found for address 0x%x", addr)
}

// findLineInProgram finds the source line for an address in a line program
func (p *Parser) findLineInProgram(lp *dwarf.LineProgram, addr uint64) *SourceLocation {
	// Iterate through line program sequences
	for {
		seq, err := lp.NextSequence()
		if err != nil || seq == nil {
			break
		}

		for {
			line, err := seq.NextLine()
			if err != nil {
				break
			}
			if line == nil {
				break
			}

			// Check if this line contains our address
			if line.IsStmt {
				// For now, return basic info
				return &SourceLocation{
					File:   line.File.Name,
					Line:   int(line.Line),
					Column: int(line.Column),
				}
			}
		}
	}

	return nil
}

// formatLocation formats a DWARF location description
func formatLocation(loc []byte) string {
	if len(loc) == 0 {
		return ""
	}

	switch loc[0] {
	case dwarf.LocExprStackValue:
		return "immediate"
	case dwarf.LocAddr:
		if len(loc) >= 9 {
			addr := binary.LittleEndian.Uint64(loc[1:])
			return fmt.Sprintf("0x%x", addr)
		}
	case dwarf.LocEnd:
		return "end"
	}

	return fmt.Sprintf("location[0x%x]", loc[0])
}

// nameDemangle attempts to demangle a name (simplified version)
func nameDemangle(name string) string {
	// Basic Rust demangling: _RNv... -> original name
	if len(name) > 4 && name[:4] == "_RNv" {
		// For now, just return the original
		return name
	}
	return name
}

// HasDebugInfo returns true if the binary contains DWARF debug information
func (p *Parser) HasDebugInfo() bool {
	return p.data != nil
}

// BinaryType returns the type of binary being parsed
func (p *Parser) BinaryType() string {
	return p.binaryType
}
