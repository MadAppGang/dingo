package preprocessor

import (
	"fmt"
	"strings"
)

// StdlibRegistry maps function names to the standard library packages they belong to.
// Functions appearing in multiple packages are listed with all packages for ambiguity detection.
var StdlibRegistry = map[string][]string{
	// === os package ===
	"Chdir":          {"os"},
	"Chmod":          {"os"},
	"Chown":          {"os"},
	"Chtimes":        {"os"},
	"Clearenv":       {"os"},
	"DirFS":          {"os"},
	"Environ":        {"os"},
	"Executable":     {"os"},
	"Exit":           {"os"},
	"Expand":         {"os"},
	"ExpandEnv":      {"os"},
	"Getegid":        {"os"},
	"Getenv":         {"os"},
	"Geteuid":        {"os"},
	"Getgid":         {"os"},
	"Getgroups":      {"os"},
	"Getpagesize":    {"os"},
	"Getpid":         {"os"},
	"Getppid":        {"os"},
	"Getuid":         {"os"},
	"Getwd":          {"os"},
	"Hostname":       {"os"},
	"IsExist":        {"os"},
	"IsNotExist":     {"os"},
	"IsPathSeparator": {"os"},
	"IsPermission":   {"os"},
	"IsTimeout":      {"os"},
	"Lchown":         {"os"},
	"Link":           {"os"},
	"LookupEnv":      {"os"},
	"Lstat":          {"os"},
	"Mkdir":          {"os"},
	"MkdirAll":       {"os"},
	"MkdirTemp":      {"os"},
	"NewSyscallError": {"os"},
	// "Pipe" - ambiguous:            {"os"},
	"ReadDir":        {"os"},
	"ReadFile":       {"os"},
	"Readlink":       {"os"},
	"Remove":         {"os"},
	"RemoveAll":      {"os"},
	"Rename":         {"os"},
	"SameFile":       {"os"},
	"Setenv":         {"os"},
	"Stat":           {"os"},
	"Symlink":        {"os"},
	"TempDir":        {"os"},
	"Truncate":       {"os"},
	"Unsetenv":       {"os"},
	"UserCacheDir":   {"os"},
	"UserConfigDir":  {"os"},
	"UserHomeDir":    {"os"},
	"WriteFile":      {"os"},

	// === fmt package ===
	"Errorf":   {"fmt"},
	"Fprint":   {"fmt"},
	"Fprintf":  {"fmt"},
	"Fprintln": {"fmt"},
	"Fscan":    {"fmt"},
	"Fscanf":   {"fmt"},
	"Fscanln":  {"fmt"},
	"Print":    {"fmt"},
	"Printf":   {"fmt"},
	"Println":  {"fmt"},
	"Scan":     {"fmt"},
	"Scanf":    {"fmt"},
	"Scanln":   {"fmt"},
	"Sprint":   {"fmt"},
	"Sprintf":  {"fmt"},
	"Sprintln": {"fmt"},
	"Sscan":    {"fmt"},
	"Sscanf":   {"fmt"},
	"Sscanln":  {"fmt"},

	// === strconv package ===
	"AppendBool":    {"strconv"},
	"AppendFloat":   {"strconv"},
	"AppendInt":     {"strconv"},
	"AppendQuote":   {"strconv"},
	"AppendQuoteRune": {"strconv"},
	"AppendQuoteRuneToASCII": {"strconv"},
	"AppendQuoteRuneToGraphic": {"strconv"},
	"AppendQuoteToASCII": {"strconv"},
	"AppendQuoteToGraphic": {"strconv"},
	"AppendUint":    {"strconv"},
	"Atoi":          {"strconv"},
	"CanBackquote":  {"strconv"},
	"FormatBool":    {"strconv"},
	"FormatComplex": {"strconv"},
	"FormatFloat":   {"strconv"},
	"FormatInt":     {"strconv"},
	"FormatUint":    {"strconv"},
	"IsGraphic":     {"strconv"},
	"IsPrint":       {"strconv"},
	"Itoa":          {"strconv"},
	"ParseBool":     {"strconv"},
	"ParseComplex":  {"strconv"},
	"ParseFloat":    {"strconv"},
	"ParseInt":      {"strconv"},
	"ParseUint":     {"strconv"},
	"Quote":         {"strconv"},
	"QuoteRune":     {"strconv"},
	"QuoteRuneToASCII": {"strconv"},
	"QuoteRuneToGraphic": {"strconv"},
	"QuoteToASCII":  {"strconv"},
	"QuoteToGraphic": {"strconv"},
	"Unquote":       {"strconv"},
	"UnquoteChar":   {"strconv"},

	// === io package ===
	"Copy":       {"io"},
	"CopyBuffer": {"io"},
	"CopyN":      {"io"},
	"ReadAll":    {"io"},
	"ReadAtLeast": {"io"},
	"ReadFull":   {"io"},
	"WriteString": {"io"},

	// === encoding/json package ===
	"Compact":      {"encoding/json"},
	"HTMLEscape":   {"encoding/json"},
	"Indent":       {"encoding/json"},
	"Marshal":      {"encoding/json"},
	"MarshalIndent": {"encoding/json"},
	"Unmarshal":    {"encoding/json"},
	"Valid":        {"encoding/json"},

	// === net/http package ===
	"CanonicalHeaderKey": {"net/http"},
	"DetectContentType":  {"net/http"},
	"Error":              {"net/http"},
	"Get":                {"net/http", "sync"},
	"Handle":             {"net/http"},
	"HandleFunc":         {"net/http"},
	"Head":               {"net/http"},
	"ListenAndServe":     {"net/http"},
	"ListenAndServeTLS":  {"net/http"},
	"MaxBytesReader":     {"net/http"},
	"NewRequest":         {"net/http"},
	"NewRequestWithContext": {"net/http"},
	"NotFound":           {"net/http"},
	"ParseHTTPVersion":   {"net/http"},
	"ParseTime":          {"net/http"},
	"Post":               {"net/http"},
	"PostForm":           {"net/http"},
	"ProxyFromEnvironment": {"net/http"},
	"ProxyURL":           {"net/http"},
	"ReadRequest":        {"net/http"},
	"ReadResponse":       {"net/http"},
	"Redirect":           {"net/http"},
	"Serve":              {"net/http"},
	"ServeContent":       {"net/http"},
	"ServeFile":          {"net/http"},
	"ServeTLS":           {"net/http"},
	"SetCookie":          {"net/http"},
	"StatusText":         {"net/http"},

	// === sync package ===
	"NewCond": {"sync"},

	// === time package ===
	"After":      {"time"},
	"AfterFunc":  {"time"},
	"Date":       {"time"},
	"NewTicker":  {"time"},
	"NewTimer":   {"time"},
	"Now":        {"time"},
	"Parse":      {"time"},
	"ParseDuration": {"time"},
	"ParseInLocation": {"time"},
	"Since":      {"time"},
	"Sleep":      {"time"},
	"Tick":       {"time"},
	"Unix":       {"time"},
	"UnixMicro":  {"time"},
	"UnixMilli":  {"time"},
	"Until":      {"time"},

	// === errors package ===
	"As":     {"errors"},
	"Is":     {"errors"},
	// "Join" - ambiguous with strings, bytes, filepath, path
	// "New" - ambiguous with rand, sync
	"Unwrap": {"errors"},

	// === strings package ===
	// Many functions ambiguous with bytes package - see ambiguous section
	"Clone":        {"strings"},
	"NewReplacer":  {"strings"},

	// === bytes package ===
	"Equal":        {"bytes"},
	"NewBuffer":    {"bytes"},
	"NewBufferString": {"bytes"},
	"Runes":        {"bytes"},

	// === path/filepath package ===
	// "Abs" - ambiguous:           {"path/filepath"},
	"EvalSymlinks": {"path/filepath"},
	"FromSlash":    {"path/filepath"},
	"Glob":         {"path/filepath"},
	"IsLocal":      {"path/filepath"},
	"Rel":          {"path/filepath"},
	"SplitList":    {"path/filepath"},
	"ToSlash":      {"path/filepath"},
	"VolumeName":   {"path/filepath"},
	"Walk":         {"path/filepath"},
	"WalkDir":      {"path/filepath"},

	// === path package ===
	// Note: Many path functions overlap with filepath

	// === regexp package ===
	"Compile":         {"regexp"},
	"CompilePOSIX":    {"regexp"},
	"MatchReader":     {"regexp"},
	"MatchString":     {"regexp"},
	"QuoteMeta":       {"regexp"},

	// === sort package ===
	"Float64s":       {"sort"},
	"Float64sAreSorted": {"sort"},
	"Ints":           {"sort"},
	"IntsAreSorted":  {"sort"},
	"IsSorted":       {"sort"},
	"SearchFloat64s": {"sort"},
	"SearchInts":     {"sort"},
	"SearchStrings":  {"sort"},
	"Slice":          {"sort"},
	"SliceIsSorted":  {"sort"},
	"SliceStable":    {"sort"},
	"Stable":         {"sort"},
	"Strings":        {"sort"},
	"StringsAreSorted": {"sort"},

	// === math package ===
	"Acos":    {"math"},
	"Acosh":   {"math"},
	"Asin":    {"math"},
	"Asinh":   {"math"},
	"Atan":    {"math"},
	"Atan2":   {"math"},
	"Atanh":   {"math"},
	"Cbrt":    {"math"},
	"Ceil":    {"math"},
	"Copysign": {"math"},
	"Cos":     {"math"},
	"Cosh":    {"math"},
	"Dim":     {"math"},
	"Erf":     {"math"},
	"Erfc":    {"math"},
	"Erfcinv": {"math"},
	"Erfinv":  {"math"},
	"Exp":     {"math"},
	"Exp2":    {"math"},
	"Expm1":   {"math"},
	"FMA":     {"math"},
	"Float32bits": {"math"},
	"Float32frombits": {"math"},
	"Float64bits": {"math"},
	"Float64frombits": {"math"},
	"Floor":   {"math"},
	"Frexp":   {"math"},
	"Gamma":   {"math"},
	"Hypot":   {"math"},
	"Ilogb":   {"math"},
	"Inf":     {"math"},
	"IsInf":   {"math"},
	"IsNaN":   {"math"},
	"J0":      {"math"},
	"J1":      {"math"},
	"Jn":      {"math"},
	"Ldexp":   {"math"},
	"Lgamma":  {"math"},
	"Log":     {"math"},
	"Log10":   {"math"},
	"Log1p":   {"math"},
	"Log2":    {"math"},
	"Logb":    {"math"},
	"Max":     {"math"},
	"Min":     {"math"},
	"Mod":     {"math"},
	"Modf":    {"math"},
	"NaN":     {"math"},
	"Nextafter": {"math"},
	"Nextafter32": {"math"},
	"Pow":     {"math"},
	"Pow10":   {"math"},
	"Remainder": {"math"},
	"Round":   {"math"},
	"RoundToEven": {"math"},
	"Signbit": {"math"},
	"Sin":     {"math"},
	"Sincos":  {"math"},
	"Sinh":    {"math"},
	"Sqrt":    {"math"},
	"Tan":     {"math"},
	"Tanh":    {"math"},
	"Trunc":   {"math"},
	"Y0":      {"math"},
	"Y1":      {"math"},
	"Yn":      {"math"},

	// === math/rand package ===
	"ExpFloat64": {"math/rand"},
	"Float32":    {"math/rand"},
	"Float64":    {"math/rand"},
	"Int":        {"math/rand"},
	"Int31":      {"math/rand"},
	"Int31n":     {"math/rand"},
	"Int63":      {"math/rand"},
	"Int63n":     {"math/rand"},
	"Intn":       {"math/rand"},
	"NormFloat64": {"math/rand"},
	"Perm":       {"math/rand"},
	"Seed":       {"math/rand"},
	"Shuffle":    {"math/rand"},
	"Uint32":     {"math/rand"},
	"Uint64":     {"math/rand"},

	// === context package ===
	"Background":    {"context"},
	"TODO":          {"context"},
	"WithCancel":    {"context"},
	"WithDeadline":  {"context"},
	"WithTimeout":   {"context"},
	"WithValue":     {"context"},

	// === log package ===
	"Fatal":   {"log"},
	"Fatalf":  {"log"},
	"Fatalln": {"log"},
	"Flags":   {"log"},
	"Output":  {"log"},
	"Panic":   {"log"},
	"Panicf":  {"log"},
	"Panicln": {"log"},
	"Prefix":  {"log"},
	"SetFlags": {"log"},
	"SetOutput": {"log"},
	"SetPrefix": {"log"},

	// === net package ===
	"Dial":          {"net"},
	"DialIP":        {"net"},
	"DialTCP":       {"net"},
	"DialTimeout":   {"net"},
	"DialUDP":       {"net"},
	"DialUnix":      {"net"},
	"FileConn":      {"net"},
	"FileListener":  {"net"},
	"FilePacketConn": {"net"},
	"InterfaceAddrs": {"net"},
	"InterfaceByIndex": {"net"},
	"InterfaceByName": {"net"},
	"Interfaces":    {"net"},
	"JoinHostPort":  {"net"},
	"Listen":        {"net"},
	"ListenIP":      {"net"},
	"ListenMulticastUDP": {"net"},
	"ListenPacket":  {"net"},
	"ListenTCP":     {"net"},
	"ListenUDP":     {"net"},
	"ListenUnix":    {"net"},
	"ListenUnixgram": {"net"},
	"LookupAddr":    {"net"},
	"LookupCNAME":   {"net"},
	"LookupHost":    {"net"},
	"LookupIP":      {"net"},
	"LookupMX":      {"net"},
	"LookupNS":      {"net"},
	"LookupPort":    {"net"},
	"LookupSRV":     {"net"},
	"LookupTXT":     {"net"},
	"ParseCIDR":     {"net"},
	"ParseIP":       {"net"},
	"ParseMAC":      {"net"},
	"ResolveIPAddr": {"net"},
	"ResolveTCPAddr": {"net"},
	"ResolveUDPAddr": {"net"},
	"ResolveUnixAddr": {"net"},
	"SplitHostPort": {"net"},

	// === Ambiguous functions (appear in multiple packages) ===
	"Open":  {"os", "net"},
	"Close": {"os", "io", "net"},
	"Write": {"io", "os", "bufio"},
	"Read":  {"io", "os", "bufio", "math/rand"},
	"Pipe":  {"net", "os", "io"},
	"Join":  {"strings", "bytes", "path/filepath", "path"},
	"Compare": {"strings", "bytes"},
	"Contains": {"strings", "bytes"},
	"ContainsAny": {"strings", "bytes"},
	"ContainsRune": {"strings", "bytes"},
	"Count": {"strings", "bytes"},
	"Cut": {"strings", "bytes"},
	"CutPrefix": {"strings", "bytes"},
	"CutSuffix": {"strings", "bytes"},
	"EqualFold": {"strings", "bytes"},
	"Fields": {"strings", "bytes"},
	"FieldsFunc": {"strings", "bytes"},
	"HasPrefix": {"strings", "bytes", "path/filepath"},
	"HasSuffix": {"strings", "bytes"},
	"Index": {"strings", "bytes"},
	"IndexAny": {"strings", "bytes"},
	"IndexByte": {"strings", "bytes"},
	"IndexFunc": {"strings", "bytes"},
	"IndexRune": {"strings", "bytes"},
	"LastIndex": {"strings", "bytes"},
	"LastIndexAny": {"strings", "bytes"},
	"LastIndexByte": {"strings", "bytes"},
	"LastIndexFunc": {"strings", "bytes"},
	"Map": {"strings", "bytes"},
	"NewReader": {"strings", "bytes"},
	"Repeat": {"strings", "bytes"},
	"Replace": {"strings", "bytes"},
	"ReplaceAll": {"strings", "bytes"},
	"Split": {"strings", "bytes", "path/filepath", "path"},
	"SplitAfter": {"strings", "bytes"},
	"SplitAfterN": {"strings", "bytes"},
	"SplitN": {"strings", "bytes"},
	"Title": {"strings", "bytes"},
	"ToLower": {"strings", "bytes"},
	"ToLowerSpecial": {"strings", "bytes"},
	"ToTitle": {"strings", "bytes"},
	"ToTitleSpecial": {"strings", "bytes"},
	"ToUpper": {"strings", "bytes"},
	"ToUpperSpecial": {"strings", "bytes"},
	"ToValidUTF8": {"strings", "bytes"},
	"Trim": {"strings", "bytes"},
	"TrimFunc": {"strings", "bytes"},
	"TrimLeft": {"strings", "bytes"},
	"TrimLeftFunc": {"strings", "bytes"},
	"TrimPrefix": {"strings", "bytes"},
	"TrimRight": {"strings", "bytes"},
	"TrimRightFunc": {"strings", "bytes"},
	"TrimSpace": {"strings", "bytes"},
	"TrimSuffix": {"strings", "bytes"},
	"Base": {"path/filepath", "path"},
	"Clean": {"path/filepath", "path"},
	"Dir": {"path/filepath", "path"},
	"Ext": {"path/filepath", "path"},
	"IsAbs": {"path/filepath", "path"},
	"Match": {"path/filepath", "path", "regexp"},
	"Search": {"sort", "regexp"},
	"New": {"math/rand", "errors", "sync"},
	"Abs": {"math", "path/filepath"},
}

// AmbiguousFunctionError is returned when a function name exists in multiple packages
// and cannot be automatically inferred.
type AmbiguousFunctionError struct {
	Function string
	Packages []string
}

// Error returns a helpful error message with fix-it hints.
func (e *AmbiguousFunctionError) Error() string {
	// Sort packages for consistent error messages
	pkgs := make([]string, len(e.Packages))
	copy(pkgs, e.Packages)

	var fixExamples string
	if len(pkgs) >= 2 {
		fixExamples = fmt.Sprintf("%s.%s or %s.%s",
			pkgs[0], e.Function,
			pkgs[1], e.Function)
	} else {
		fixExamples = fmt.Sprintf("%s.%s", pkgs[0], e.Function)
	}

	return fmt.Sprintf(
		"ambiguous function '%s' could be from: %s\n"+
		"Fix: Use qualified call (e.g., %s)",
		e.Function,
		strings.Join(pkgs, ", "),
		fixExamples,
	)
}

// GetPackageForFunction returns the package name for a given function.
// Returns:
//   - (packageName, nil) if the function uniquely belongs to one package
//   - ("", AmbiguousFunctionError) if the function exists in multiple packages
//   - ("", nil) if the function is not in the stdlib registry
func GetPackageForFunction(funcName string) (string, error) {
	pkgs, exists := StdlibRegistry[funcName]

	if !exists {
		// Not a stdlib function (could be user-defined)
		return "", nil
	}

	if len(pkgs) == 0 {
		// Should never happen, but handle defensively
		return "", nil
	}

	if len(pkgs) > 1 {
		// Ambiguous: function exists in multiple packages
		return "", &AmbiguousFunctionError{
			Function: funcName,
			Packages: pkgs,
		}
	}

	// Unique mapping
	return pkgs[0], nil
}

// IsStdlibFunction returns true if the function name is registered in the stdlib.
func IsStdlibFunction(funcName string) bool {
	_, exists := StdlibRegistry[funcName]
	return exists
}

// GetAllPackages returns all unique packages in the registry.
func GetAllPackages() []string {
	pkgSet := make(map[string]bool)
	for _, pkgs := range StdlibRegistry {
		for _, pkg := range pkgs {
			pkgSet[pkg] = true
		}
	}

	result := make([]string, 0, len(pkgSet))
	for pkg := range pkgSet {
		result = append(result, pkg)
	}
	return result
}

// GetFunctionCount returns the total number of registered functions.
func GetFunctionCount() int {
	return len(StdlibRegistry)
}

// GetAmbiguousFunctions returns all function names that exist in multiple packages.
func GetAmbiguousFunctions() []string {
	var result []string
	for funcName, pkgs := range StdlibRegistry {
		if len(pkgs) > 1 {
			result = append(result, funcName)
		}
	}
	return result
}
