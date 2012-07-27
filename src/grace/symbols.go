package grace

import (
  "debug/elf"
  "debug/dwarf"
  "strconv"
  "strings"
  "fmt"
)


func contains(haystack, needle InstantiatedRange) bool {
  return haystack.Low() <= needle.Low() && haystack.High() >= needle.High()
}

func atoi(a string) (ret int) {
  ret, er := strconv.Atoi(a)
  if er != nil {
    ret = 0
  }
  return
}

func extractTextSectionOffset(file *elf.File) uint64 {
  var section *elf.Section
  for _, section = range file.Sections {
    if section.Name == ".text" {
      return section.Addr
    }
  }
  return 0
}

// ExtractSymbolTable attempts to parse the DWARF section of a binary and return
// a symbol table. This currently just supports file names, and function 
// definitions
func ExtractSymbolTable(binary string, offset uint64) (*SymbolTable, error) {
  files := make(SymbolTable)

  f, err := elf.Open(binary)
  if err != nil {
    return nil, err
  }
  defer f.Close()

  dwarfs, err := f.DWARF()
  if err != nil {
    return nil, err
  }

  dwarfReader := dwarfs.Reader()
  for {
    entry, _ := dwarfReader.Next()
    if entry == nil {
      break
    }

    /* TODO: This is all by value, make this references */
    // For now, all we need are files and functions
    switch entry.Tag {

    case dwarf.TagCompileUnit:
      file := extractFile(entry)
      // file.Lowpc += offset
      // file.Highpc += offset
      name := file.Filename

      if name == "" {
        continue
      }

      files[name] = file

    /* TODO: Stupid inefficient */
    case dwarf.TagSubprogram:
      fun := extractFunction(entry)
      // fun.Highpc += offset
      // fun.Lowpc += offset
      for _, v := range files {
        if contains(v, fun) {
          v.Functions[fun.Name] = fun
        }
      }
    }
  }

  return &files, nil

}

// extractFunction Turns a DWARF function entry to a grace.CompiledFunction
func extractFunction(entry *dwarf.Entry) (fun CompiledFunction) {
  fun = CompiledFunction { }
  for _,field := range entry.Field {
    switch field.Attr {
    case dwarf.AttrName:
      fun.Name = field.Val.(string)
    case dwarf.AttrDeclLine:
      fun.Lineno = int(field.Val.(int64))
    case dwarf.AttrHighpc:
      fun.Highpc = field.Val.(uint64)
    case dwarf.AttrLowpc:
      fun.Lowpc = field.Val.(uint64)
    }
  }
  return
}

// extractFile turns a dwarf.Entry into a grace.CompiledFile
func extractFile(entry *dwarf.Entry) (file CompiledFile) {
  file = CompiledFile { 
    Functions: make(map[string]CompiledFunction),
  }
  for _,field := range entry.Field {
    switch field.Attr {
    case dwarf.AttrName:
      file.Filename = field.Val.(string)
    case dwarf.AttrLowpc:
      file.Lowpc = field.Val.(uint64)
    case dwarf.AttrHighpc:
      file.Highpc = field.Val.(uint64)
    }
  }
  return
}

type symbolPath struct {
  file, function string
  line int

}

func TestResolveSymbol (p *Process) {
  var ans uint64
  var sym string

  sym = "foo.c:32"
  ans, err := p.resolveSymbol(sym)
  fmt.Printf("%s = %x(%s)\n", sym, ans, err.Error())

  sym = "0x80000000"
  ans, _ = p.resolveSymbol(sym)
  fmt.Printf("%s = %x\n",sym, ans)

  sym = "foo.c:bar"
  ans, _ = p.resolveSymbol(sym)
  fmt.Printf("%s = %x\n",sym, ans)

  sym = "foo.c:10"
  ans, _ = p.resolveSymbol(sym)
  fmt.Printf("%s = %x\n",sym, ans)

  sym = "WebCore::ScrollView::printFoo"
  ans, _ = p.resolveSymbol(sym)
  fmt.Printf("%s = %x\n",sym, ans)

}

func isAlnum(s string) bool {
  if len(s) == 0 {
    return false
  }
  if len(s) > 2 && s[:2] == "0x" {
    s = s[2:]
  }
  for _, c := range s {
    if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
      return false
    }
  }
  return true
}

type symbolMode int
const (
  modeOther symbolMode = iota
  modeCpp
)

// symstringToTokens splits the symbol reference along ':' and returns the mode
// the symbol is likely in. (i.e. cpp or other)
func symstringToTokens(sym string) ([]string, symbolMode) {
  var mode symbolMode = modeOther
  if strings.Contains(sym, "::") {
    mode = modeCpp
  }
  return strings.Fields(strings.Replace(sym, ":", " ", -1)), mode
}

// reverseSlice does exactly what it promises
func reverseSlice(tokens []string) {
  tokensLen := len(tokens)
  for i := 0; i < tokensLen/2; i += 1 {
    tokens[i], tokens[tokensLen-1-i] = tokens[tokensLen-1-i], tokens[i]
  }
}

type symbolLocation struct {
    fileName string
    namespaceName string
    className string
    funcName string
    lineNumber int
}

type locationError int
const (
  formatNoError locationError = iota
  formatError
  missingDWARF
  symbolNotFound
  unsupported
)
func (e locationError) Error() string {
  switch e {
  case formatNoError: return "everything is okay."
  case formatError: return "symbol isn't formatted correctly"
  case missingDWARF: return "binary is missing a DWARF section. (needed for symbol lookup)"
  case symbolNotFound: return "symbol could not be found in the symbol table"
  case unsupported: return "symbol format not supported"
  }
  return "Unknown symbol resolution error"
}

// symstringToLoc takes a fuzzy 'human-readable' description of a location in
// an executable and parses it into a detailed struct.
func symstringToLoc(symstring string) (*symbolLocation, error) {
  loc := new(symbolLocation)

  tokens, mode := symstringToTokens(symstring)

  // Reverse the tokens to make popping off the stack easier
  reverseSlice(tokens)

  // If last thing was numeric, it's likely a line number and the first is a
  // filename. 
  if isAlnum(tokens[0]) {
    loc.lineNumber, _ = strconv.Atoi(tokens[0])
    loc.fileName = tokens[1]

    return loc, nil
  }

  // Regardless of mode, we get the function name
  loc.funcName = tokens[0]
  tokens = tokens[1:]

  // If parsing a C++ reference, get the class+namespace
  if mode == modeCpp {
    loc.className = tokens[0]
    if len(tokens) > 1 {
      loc.namespaceName = tokens[1]
    }
  } else {
    loc.fileName = tokens[0]
  }

  return loc, nil
}

func locToOffset(p *Process, loc *symbolLocation) (uint64, error) {
  if loc.fileName != "" {
    file, ok := (*p.DebugSymbols)[loc.fileName]
    if ! ok {
      return 0, symbolNotFound
    }

    if loc.lineNumber > 0 {
      // First check if any of the functions start on that number, otherwise,
      // need to add support for it
      for _, proc := range file.Functions {
        if proc.Lineno == loc.lineNumber {
          return proc.Lowpc, nil
        }
      }
      return 0, unsupported
    }

    proc, ok := file.Functions[loc.funcName]
    if ! ok {
      return 0, symbolNotFound
    }

    return proc.Lowpc, nil
  }

  return 0, symbolNotFound
}

// resolveSymbol attempts to take a fuzzy human-readable definition of a place
// in a binary and resolve that to an actual address. The following are intended
// to be supported: "0x08004014", "file.c:functionFoo",
// "CppNs::CppClass::CppFunc"
func (p *Process) resolveSymbol(sym string) (uint64, error) {

  /* Just an address */
  if isAlnum(sym) {
    return strconv.ParseUint(sym, 0, 64)
  }

  if p.DebugSymbols == nil {
    return 0, missingDWARF
  }

  loc, _ := symstringToLoc(sym)

  addr, err := locToOffset(p, loc)

  return addr, err
}
