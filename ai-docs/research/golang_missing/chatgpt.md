Most Desired Missing Features in Go (Proposals and Discussions)

Go’s simplicity and minimalism are often cited as strengths, but many developers have proposed adding new language features to improve ergonomics, reduce boilerplate, and enhance safety. Over the years, numerous proposals on Go’s official issue tracker have debated such feature additions. Below we outline the top features commonly requested – along with the proposals and outcomes – focusing on language syntax/semantics (not libraries or tools). All of these were either rejected or remain unresolved after extensive discussion.

Improved Error Handling Syntax

One of the loudest complaints has been Go’s verbose error-handling pattern. Code that calls functions must repeatedly check if err != nil { ... }, which some find repetitive. To streamline this, Go team members and the community put forward several ideas:
	•	Built-in try for early error returns: In 2019, a proposal by Go team members introduced a built-in try function to automatically propagate errors ￼. The idea was that instead of writing an if err != nil block for each call, one could write f := try(os.Open(file)), and try would return from the enclosing function if an error occurred ￼ ￼. This aimed to eliminate boilerplate if/err blocks and handle the common case of propagating the error up.
	•	Option for an error-check operator ?: In 2024, another draft proposal suggested a Rust-inspired ? operator to simplify error handling ￼ ￼. This syntax would allow writing, for example, x := strconv.Atoi(a) ? instead of an if err != nil block, implicitly returning the error if one occurred. Small user studies indicated most Go developers could guess what the ? meant ￼, giving some hope that it was a intuitive change.

Despite these attempts, none of the proposed syntactic sugar gained broad enough consensus to be adopted. The Go team ultimately announced that they would stop pursuing new error-handling syntax, after “three full-fledged proposals by the Go team and literally hundreds of community proposals” all failed to achieve agreement ￼ ￼. In a 2025 Go blog post, they noted that Go already has a “perfectly fine” error-handling model; introducing a second, different mechanism without overwhelming support would likely do more harm than good. They emphasize that not adding another way to handle errors aligns with Go’s design philosophy of having one obvious way to do things ￼ ￼. For now, error handling in Go will remain as-is, perhaps aided by improvements in tooling rather than language syntax.

Ternary and Null-Coalescing Operators

Another much-missed convenience is a ternary conditional operator (the condition ? trueExpr : falseExpr syntax found in C, Java, etc.). Go deliberately omitted the ternary ?:, so developers must use an if/else statement even for simple value selection. Over the years, multiple proposals have argued that a ternary operator would make code more concise and eliminate minor boilerplate. Proponents note it’s a familiar feature from other languages and can improve readability for simple conditional assignments ￼. For example, instead of:

if n > 1 {
    fmt.Printf("friend*s*\n")
} else {
    fmt.Printf("friend\n")
}

one could write fmt.Printf("friend%s\n", n>1 ? "s" : "") to pluralize a word. This “missing” ?: operator has been requested in Go since at least 2010, but the Go designers have consistently declined it. The official FAQ explains that ?: tends to encourage overly complex expressions; they found that an explicit if/else, though more verbose, is “unquestionably clearer.” The language authors felt that having one form of conditional (the if statement) is sufficient ￼. Indeed, all formal proposals to add a ternary operator (including several filed during the Go 2 planning discussions) were closed as “not planned”, citing the clarity and simplicity of the status quo.

Similarly, Go lacks a null-coalescing operator (often ?? in other languages) to provide default values. A proposal in 2020 suggested adding ?? to return the first non-zero value (analogous to a “or-else” for defaults). For example, one could write:

port := os.Getenv("PORT") ?? DefaultPort

to use a default if an environment variable is empty ￼. This would streamline the common pattern of checking a returned value and setting a fallback (which currently requires an if or two lines of code). Like the ternary, the ?? operator was intended to reduce boilerplate and make intent explicit (“use this value or else that one” in one expression). Despite some support, this idea was also not adopted – it was closed without implementation ￼. The rationale is akin to the ternary debate: the Go team prefers not to introduce new expression forms when the existing syntax (if/else or the comma-ok idiom) can handle the logic, even if a bit more verbosely.

Summary: Ternary (?:) and coalescing (??) operators are common in other languages to make code concise, but Go has chosen to avoid them. Proposals highlighting their benefit in reducing repetition have been rejected in favor of keeping conditional logic spelled out with clear if/else statements ￼ ￼.

Shorter Anonymous Function Syntax (Lambda “Arrow” Functions)

Go allows literal function closures, but the syntax can be clunky for simple cases – one must write the full func keyword, parameter list with types, and explicit return types if needed. For example:

// Using a closure to add two floats:
var _ = compute(func(a, b float64) float64 { return a + b })

Many modern languages provide a lightweight lambda syntax (sometimes using => or other notation) that infers parameter types from context. There has been a long-running proposal to introduce a short function literal syntax in Go to address this ￼. The idea is that if the compiler already knows the expected function signature (from assignment or call context), the anonymous function could be written more succinctly. For instance, in Scala or Rust one can write compute((x, y) => x + y) instead of repeating the types ￼. Go could allow something similar, e.g. compute(|x, y| x + y) or another concise form, where x and y are understood to be floats in this context.

This proposal (sometimes dubbed “arrow functions” in Go discussions) has received huge community interest. It’s been discussed since 2017 and accumulated hundreds of comments and several suggested syntaxes ￼. The benefit would be to remove boilerplate in functional patterns – making it easier to pass small callback functions or define one-off mappers, etc., without sacrificing type safety. As of 2025, however, no final decision has been made. The proposal is still open and under review by the language committee ￼. The delay reflects caution: the Go team is weighing whether the convenience of a new lambda syntax outweighs the added complexity in the language. In short, a concise lambda syntax is missing in Go today, but it’s one of the few proposals still actively on the table rather than outright rejected.

Pattern Matching Constructs

Beyond the basic switch statement, some developers wish for more powerful pattern matching in Go. In languages like Rust, Swift, or Haskell, pattern matching allows a single match expression to handle multiple conditions (and even destructure data) with compile-time exhaustiveness checking. Go’s switch can check a value against cases, but it lacks destructuring and doesn’t require covering all possible cases (and types in a type-switch).

There have been proposals to introduce a new match expression or enhance switch to cover these use cases. For example, one 2021 proposal suggested a match statement that can return a value and enforce handling of all cases (with a _ default case for “else”) ￼ ￼. An illustrative snippet from that proposal:

result := match number {
    0  => { return "Zero" }
    10 => { return "Positive" }
    _  => { return "Other" }
}

This would act as an expression (yielding a value to result), unlike Go’s existing switch which is statement-only. More advanced versions even imagined matching on multiple variables or on the types of interface values in a single construct ￼ (kind of combining a type-switch with value conditions). The developer experience benefits would be clearer and less repetitive branching logic, and potentially compile-time checks that all meaningful cases are handled.

However, Go has not adopted full pattern matching. Such proposals haven’t gained traction, partly because they overlap with what can be done with switch + if-guards, and partly because they are most useful in tandem with a concept Go also lacks – sum types (closed enums). Without algebraic sum types (discussed next), pattern matching is less powerful; and adding both features would significantly change Go’s simplicity. For now, Go continues to use switch (and if/else chains) for pattern-like logic, without the exhaustive checking found in other languages. Enthusiasts have occasionally emulated pattern matching via libraries or code generation, but there’s no native support.

Sum Types and Enumerations (Algebraic Data Types)

Related to pattern matching is the desire for sum types – also known as discriminated unions or sealed variants. A sum type allows a variable to hold one of a fixed set of types, typically with a tag to indicate which. For example, in Rust you might have:

enum Result<T> { Ok(T), Err(String) }

which can be either an Ok carrying a value or an Err carrying a message. In Go, the closest equivalent is an interface{} or a union of interfaces, but Go interfaces are open (any type implementing the interface could appear) and there’s no compiler enforcement of handling all variants. The lack of true sum types means Go developers often resort to booleans alongside values, empty interfaces with type assertions, or manual “tag” fields in structs – patterns that involve more boilerplate and are error-prone.

The Go community has long discussed introducing sum types or closed enums. One rough proposal (never formally accepted) was to allow defining an interface that is explicitly limited to certain concrete types (effectively a closed sum type) ￼ ￼. More recently, a 2022 proposal suggested “union” types using a new syntax (e.g. type Result switch { case Ok: T; case Err: string }) which would let you directly declare a discriminated union type with its variants and payload types ￼ ￼. The goal of these proposals is to enable first-class enums/sum types that the compiler knows about, bringing benefits like exhaustive matching (the compiler can force you to handle all cases) and eliminating the need to manually define a lot of wrapper types and interface implementations. As one developer noted, “matching on these enums is usually exhaustive – you must list all the cases… This leads to useful type safety… Go doesn’t have these; interfaces don’t provide exhaustiveness and consumers of your library can even add further cases.” ￼ ￼ In other words, a closed set of variants would make certain code safer and less verbose by construction.

Despite the clear developer interest (especially from folks with functional programming backgrounds), Go has not added sum types or built-in enums. The changes required are quite fundamental and carry a cost in complexity. The Go team has expressed concerns that many use-cases for sum types can be handled with interfaces and generics – albeit with more code. For example, a proposal for union types notes that you can simulate sum types by defining an interface and several struct types, but “doing so requires a large amount of code… and it is impossible to discover the set of variants dynamically” (since any type could implement that interface) ￼. In short, the boilerplate and runtime checks increase. That proposal and earlier ones (e.g. for “typed enums”) have not been accepted, likely because the Go team is very cautious about such far-reaching extensions. They would prefer to see if generics (added in Go 1.18) combined with interfaces might address some problems first. Indeed, Go’s new type parameters allow constrained unions of types in type declarations (using the | operator in interface constraints), but those are compile-time only and don’t create a new sum type at runtime. Thus, as of 2025, true algebraic sum types and built-in enumerations remain absent from Go.

(It’s worth noting that the lack of a dedicated enum keyword in Go is also a common gripe. Go uses iota and constants as a workaround for enums, which works but doesn’t provide the safety of a closed set of values. Proposals to add a more robust enum syntax have been floated ￼, but any such change faces Go 1 compatibility hurdles and have not advanced.)

Function Overloading and Default Parameters

In line with Go’s philosophy of simplicity, the language does not support function overloading (having multiple functions with the same name but different parameter types or counts). Every function name in a scope must be unique. While this eliminates any ambiguity about which function is called, it means developers sometimes create clumsily named variants (e.g. PrintString, PrintInt, etc.) or use interface{} to accept “any” type and then do a type switch internally. Many newcomers from languages like Java or C++ find the lack of overloading surprising. Indeed, proposals have asked whether Go 2 could allow basic function overloading or at least method overloading for types ￼. However, the Go team’s stance has been that overloading doesn’t buy enough to outweigh the added complexity in the language and tooling. As one commentary puts it, Go’s design emphasizes explicitness. The compiler won’t pick a version of a function for you; you must decide on distinct function names or use generics. The consequence is sometimes more verbose APIs, but with the upside of clearer code. For example, rather than one Add(x, y) that magically works for int or float, you might have to call AddInt(x, y) vs AddFloat(x, y) – making it obvious which is being used ￼ ￼. This design was intentional.

Likewise, Go does not have default parameter values (optional arguments). In other languages, one can define a function where some parameters have defaults, making them optional to the caller. In Go, every call must provide all arguments, or else the function must be overloaded with another version that supplies the default. This feature omission was “a deliberate simplification”, according to Go’s designers ￼. Experience in other languages showed that default parameters can lead to APIs that grow unwieldy: it becomes too easy to tack on new parameters with defaults, resulting in functions that have many combinations of arguments, some of which might not even make sense together. Instead, Go encourages either writing multiple functions (for each common combination of options) or using a single struct parameter if there are many options. This forces API designers to think carefully about naming and organization, rather than casually adding parameters. As the Go team put it: requiring distinct function names or parameter structs leads to “a clearer API that is easier to understand” ￼ ￼. While it does create a bit more upfront code, it avoids the confusion of implicit parameters. A mitigating factor is that Go supports variadic functions, which cover some scenarios (you can pass an optional list of values as a slice, for example).

In 2024, a formal proposal to introduce default function arguments was filed – highlighting that it could reduce boilerplate in cases where most callers use the same values ￼ ￼. However, it was closed as “not planned”, with the language committee echoing the original rationale: it would complicate function declarations and potentially encourage API design misuse. Thus, neither function overloading nor default params are available in Go, and proposals to add them have been turned down in favor of keeping function signatures explicit.

Conclusion

All the features above share a common theme: they aim to make coding in Go more convenient or less error-prone by adding language-level support for things that otherwise require verbose patterns. Generics was one major feature in this category that Go eventually adopted (after years of debate) to reduce boilerplate in data structures and algorithms. But for other requests – improved error handling, ternary and coalescing operators, concise lambdas, pattern matching, sum types, enums, overloading, etc. – the Go team has largely favored simplicity and consistency over adding new syntax. Many of these proposals were thoroughly discussed in the Go community (as evidenced by the official GitHub issues and design docs) and then declined or indefinitely postponed. Go’s core philosophy tends to be minimalist: each feature must carry its weight in complexity. While developers miss some of these conveniences, the language designers worry that adding them could undermine Go’s trademark simplicity or introduce ambiguity.

That said, Go is not static – it has evolved (e.g. adding generics and improvements to the standard library), and some of the “missing” features remain topics of discussion. Community pressure and practical experience may yet revive certain ideas (for example, the simplified lambda syntax is still under active consideration). For now, Go programmers often rely on idiomatic workarounds: code generation, editor snippets, or design patterns that compensate for the lack of a feature. The ongoing conversations in proposals and forums show a healthy debate between convenience and simplicity. Any future changes will likely be made with extreme caution, preserving Go’s ethos while trying to improve developer experience. In summary, Go prefers to omit a feature unless it’s overwhelmingly justified – which is why these frequently requested features remain absent despite their theoretical benefits, as documented in the proposal history ￼ ￼.

<!-- Sources -->


￼ ￼ ￼ ￼ ￼ ￼ ￼ ￼ ￼ ￼ ￼ ￼