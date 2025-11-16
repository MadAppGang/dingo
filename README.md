# Dingo

<p align="center">
  <img src="docs/mascot.png" alt="Dingo mascot" width="200"/>
</p>

<p align="center">
  <strong>Go, but with all the features you actually wanted.</strong>
</p>

---

## Look, I love Go. But...

You know that feeling when you're writing Go and you type `if err != nil` for the 47th time in a single file?

Or when you forget to check for nil and your production server learns what a panic feels like?

Or when you're explaining to a Rust developer why Go doesn't have sum types and they look at you like you just said "we don't believe in seatbelts"?

Yeah. That's why Dingo exists.

## What's Dingo?

Think TypeScript, but for Go.

Dingo is a language that compiles to clean, idiomatic Go code. Not some franken-runtime or a whole new ecosystem. Just better syntax that becomes regular Go.

**The pitch:** Write code with Result types, pattern matching, and null safety. Get back perfect Go code that your team can read, your tools can process, and your production servers can run at exactly the same speed.

Zero runtime overhead. Zero new dependencies. Zero "what's this weird thing in my transpiled code?"

## Why should you care?

Here's what the Go community has been begging for since 2009:

| What developers want | How badly | What Dingo gives you |
|---------------------|-----------|---------------------|
| Stop typing `if err != nil` every 3 lines | ⭐⭐⭐⭐⭐ | `Result<T, E>` + the `?` operator |
| Stop shipping nil pointer panics | ⭐⭐⭐⭐⭐ | `Option<T>` type that the compiler actually checks |
| Sum types (seriously, it's been 15 years) | ⭐⭐⭐⭐⭐ 996+ 👍 | `enum` with pattern matching |
| Enums that can't be invalid | ⭐⭐⭐⭐⭐ 900+ 👍 | Type-safe enums with exhaustiveness |
| Lambda functions that don't take 4 lines | ⭐⭐⭐⭐ 750+ 👍 | `\|x\| x * 2` like a normal language |

The Go team has valid reasons for rejecting these features. They're not wrong about simplicity.

But here's the thing: **Dingo doesn't change Go.** We just compile to it.

Want sum types? Great. They become tagged structs in Go.
Want the `?` operator? Cool. It becomes `if err != nil` checks.
Want pattern matching? Done. It's a switch statement underneath.

Your Go code stays pure. Your Dingo code stays sane.

---

## Show me code or I'm leaving

Fair enough.

### The "if err != nil" problem

**What you write in Go today:**

```go
func processOrder(orderID string) (*Order, error) {
    order, err := fetchOrder(orderID)
    if err != nil {
        return nil, fmt.Errorf("fetch failed: %w", err)
    }

    validated, err := validateOrder(order)
    if err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    payment, err := processPayment(validated)
    if err != nil {
        return nil, fmt.Errorf("payment failed: %w", err)
    }

    return payment, nil
}
```

75% of this function is error handling ceremony. The actual logic is hiding somewhere in there.

**What you write in Dingo:**

```go
func processOrder(orderID: string) -> Result<Order, Error> {
    let order = fetchOrder(orderID)?
    let validated = validateOrder(order)?
    let payment = processPayment(validated)?
    return Ok(payment)
}
```

Same safety. Same error handling. 60% less code.

The `?` just means "if this is an error, return it. Otherwise, unwrap the value and keep going." That's it. That's the entire feature.

Rust developers have been using this for 8 years. They love it so much they put it on t-shirts.

---

## The "Holy Crap" Example

Want to see something beautiful? Here's real-world Go code I found in production:

**Go (85 lines of pain):**

```go
func ProcessUserDataPipeline(userID string, options *ProcessOptions) (*UserReport, error) {
    // Fetch user
    user, err := db.GetUser(userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get user: %w", err)
    }
    if user == nil {
        return nil, errors.New("user not found")
    }

    // Get user's orders
    orders, err := db.GetOrdersForUser(user.ID)
    if err != nil {
        return nil, fmt.Errorf("failed to get orders: %w", err)
    }

    // Filter valid orders
    var validOrders []Order
    for _, order := range orders {
        if order.Status != "cancelled" && order.Total > 0 {
            validOrders = append(validOrders, order)
        }
    }

    // Calculate totals
    var totalSpent float64
    for _, order := range validOrders {
        totalSpent += order.Total
    }

    // Get user preferences
    prefs, err := db.GetPreferences(user.ID)
    if err != nil {
        return nil, fmt.Errorf("failed to get preferences: %w", err)
    }

    // Apply discount if premium
    discount := 0.0
    if prefs != nil && prefs.IsPremium {
        discount = totalSpent * 0.1
    }

    // Get shipping address
    address, err := db.GetShippingAddress(user.ID)
    if err != nil {
        return nil, fmt.Errorf("failed to get address: %w", err)
    }

    // Format city name
    cityName := "Unknown"
    if address != nil && address.City != nil {
        cityName = *address.City
    }

    // Get payment methods
    payments, err := db.GetPaymentMethods(user.ID)
    if err != nil {
        return nil, fmt.Errorf("failed to get payment methods: %w", err)
    }

    // Find default payment
    var defaultPayment *PaymentMethod
    for i := range payments {
        if payments[i].IsDefault {
            defaultPayment = &payments[i]
            break
        }
    }

    // Get recommendation score
    score, err := analytics.GetRecommendationScore(user.ID)
    if err != nil {
        // Non-critical, use default
        score = 0.0
    }

    // Build report
    report := &UserReport{
        UserID:         user.ID,
        Email:          user.Email,
        TotalSpent:     totalSpent,
        Discount:       discount,
        OrderCount:     len(validOrders),
        City:           cityName,
        HasPayment:     defaultPayment != nil,
        RecommendScore: score,
    }

    return report, nil
}
```

**Dingo (28 lines of clarity):**

```go
func ProcessUserDataPipeline(userID: string, options: ProcessOptions) -> Result<UserReport, Error> {
    let user = db.GetUser(userID)?.okOr("user not found")?

    let orders = db.GetOrdersForUser(user.ID)?
    let validOrders = orders.filter(|o| o.status != "cancelled" && o.total > 0)
    let totalSpent = validOrders.map(|o| o.total).sum()

    let prefs = db.GetPreferences(user.ID)?
    let discount = prefs.isPremium ? totalSpent * 0.1 : 0.0

    let address = db.GetShippingAddress(user.ID)?
    let cityName = address?.city?.name ?? "Unknown"

    let payments = db.GetPaymentMethods(user.ID)?
    let defaultPayment = payments.find(|p| p.isDefault)

    let score = analytics.GetRecommendationScore(user.ID).unwrapOr(0.0)

    return Ok(UserReport{
        userID: user.id,
        email: user.email,
        totalSpent: totalSpent,
        discount: discount,
        orderCount: validOrders.len(),
        city: cityName,
        hasPayment: defaultPayment.isSome(),
        recommendScore: score,
    })
}
```

**67% less code. Same functionality. Infinitely more readable.**

Look at what just happened:
- ✅ Error propagation with `?` eliminated 12 `if err != nil` blocks
- ✅ Lambda functions turned 8-line loops into one-liners
- ✅ Optional chaining `?.` replaced nested nil checks
- ✅ Ternary operator cleaned up conditional assignments
- ✅ `.filter()`, `.map()`, `.sum()` made collection operations obvious

The business logic literally jumps off the screen now. You can see what it's doing instead of drowning in error handling boilerplate.

This is what Dingo does. It takes your Go code and makes it *readable*.

---

## Features that'll make you smile

### 1. Result Type — Error handling for grown-ups

Stop returning `(value, error)` tuples and hoping callers remember to check both.

```go
func fetchUser(id: string) -> Result<User, DatabaseError> {
    if !isValidID(id) {
        return Err(DatabaseError.invalidID(id))
    }

    let user = database.query(id)
    return Ok(user)
}

// The compiler forces you to handle both cases
match fetchUser("123") {
    Ok(user) => println("Found: ${user.name}"),
    Err(error) => println("Database said no: ${error.message}")
}
```

Your function signature now tells you exactly what can go wrong. No surprises. No "oh, I didn't know this could return an error."

### 2. Error Propagation (`?`) — Because life's too short

This one's from Rust, and it's honestly genius.

```go
func getUserProfile(userID: string) -> Result<Profile, Error> {
    let user = fetchUser(userID)?          // Returns error if this fails
    let posts = fetchPosts(user.ID)?       // Or this
    let comments = fetchComments(user.ID)? // Or this

    return Ok(Profile{user, posts, comments})
}
```

Every `?` is an escape hatch. Hit an error? Jump straight to the return. Got a value? Keep going.

It's like `try/catch` but without the invisible control flow that makes you question your life choices.

### 3. Option Type — Nil pointers are a billion-dollar mistake

Tony Hoare (the guy who invented null references) literally apologized for it. Called it his "billion-dollar mistake."

We don't have to keep living with that mistake.

```go
func findUser(email: string) -> Option<User> {
    let users = db.query("SELECT * FROM users WHERE email = ?", email)
    if users.isEmpty() {
        return None
    }
    return Some(users[0])
}

// Safe navigation like it's 2024
let city = user?.address?.city?.name ?? "Unknown"

// Compiler won't let you forget to check
match findUser("test@example.com") {
    Some(user) => sendEmail(user),
    None => println("Who dis?")
}
```

Can't accidentally dereference `None`. The type system literally won't let you compile if you forget to handle it.

How many production panics would this have prevented in your codebase? I'll wait.

### 4. Pattern Matching — Switch statements grew up

Go's switch is fine. But imagine if it could do *this*:

```go
enum HttpResponse {
    Ok(body: string),
    NotFound,
    ServerError{code: int, message: string},
    Redirect(url: string)
}

func handleResponse(resp: HttpResponse) -> string {
    match resp {
        Ok(body) => "Success: ${body}",
        NotFound => "404: Not found",
        ServerError{code, message} => "Error ${code}: ${message}",
        Redirect(url) => "Redirecting to ${url}"
        // Forget a case? Compiler yells at you
    }
}
```

Exhaustiveness checking means the compiler knows every possible case. Miss one? Won't compile.

No more "oh crap, we didn't handle the timeout case" at 2 AM.

### 5. Sum Types — The #1 requested feature in Go history

996 upvotes on the Go proposal. That's not just popular, that's "the entire community is screaming for this."

```go
enum State {
    Idle,
    Loading{progress: float},
    Success{data: string},
    Error{message: string}
}

func render(state: State) -> string {
    match state {
        Idle => "Ready when you are",
        Loading{progress} if progress < 50 => "Just getting started...",
        Loading{progress} => "Loading: ${progress}%",
        Success{data} => showSuccess(data),
        Error{message} => showError(message)
    }
}
```

A value can be one of several types. The compiler tracks which one. You can't mess it up.

This is how Rust does enums. How Swift does enums. How Kotlin does sealed classes. How TypeScript does discriminated unions.

Everyone has this except Go. Until now.

### 6. Lambda Functions — Multiple styles, pick your favorite

**Rust/JavaScript style with pipes:**
```go
users.filter(|u| u.age > 18)
    .map(|u| u.name)
    .sorted()
```

**Kotlin style with braces and implicit `it`:**
```go
users.filter { it.age > 18 }
    .map { it.name }
    .sorted()
```

**Swift style with dollar-sign shortcuts:**
```go
users.filter { $0.age > 18 }
    .map { $0.name }
    .sorted()
```

**Full syntax when you need types:**
```go
users.filter(|u: User| -> bool { u.age > 18 && u.verified })
```

Compare that to Go's verbose function literals:

```go
filteredUsers := make([]User, 0)
for _, u := range users {
    if u.Age > 18 {
        filteredUsers = append(filteredUsers, u)
    }
}

names := make([]string, 0, len(filteredUsers))
for _, u := range filteredUsers {
    names = append(names, u.Name)
}

sort.Strings(names)
```

Yeah. Lambda functions are just nicer. That's the whole argument.

### 7. Null Safety Operators — Chain nil checks like a human

**The Go way:**
```go
var city string
if user != nil && user.Address != nil && user.Address.City != nil {
    city = *user.Address.City
} else {
    city = "Unknown"
}
```

**The Dingo way:**
```go
let city = user?.address?.city?.name ?? "Unknown"
```

One line. Same safety. Your eyes will thank you.

### 8. Ternary Operator — Yes, we're going there

Go rejected this. We're adding it anyway.

```go
let max = a > b ? a : b
let status = isActive ? "online" : "offline"
println("You have ${count} item${count == 1 ? "" : "s"}")
```

Every. Single. Modern. Language. Has. This.

C has it. Java has it. JavaScript has it. Python has it (kinda). Swift has it. Even PHP has it.

Go's reason for not having it? "We only need one conditional construct."

Cool. You do you, Go. We'll be over here with our one-liners.

---

## More Real-World Examples

### API Handler: Before and After

**Go (42 lines):**

```go
func HandleUserUpdate(w http.ResponseWriter, r *http.Request) {
    userID := r.URL.Query().Get("id")
    if userID == "" {
        http.Error(w, "missing user ID", http.StatusBadRequest)
        return
    }

    var updateReq UpdateRequest
    if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
        http.Error(w, "invalid JSON", http.StatusBadRequest)
        return
    }

    user, err := db.GetUser(userID)
    if err != nil {
        http.Error(w, "database error", http.StatusInternalServerError)
        return
    }
    if user == nil {
        http.Error(w, "user not found", http.StatusNotFound)
        return
    }

    if err := validateUpdate(&updateReq); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    if err := db.UpdateUser(user.ID, &updateReq); err != nil {
        http.Error(w, "update failed", http.StatusInternalServerError)
        return
    }

    updated, err := db.GetUser(user.ID)
    if err != nil {
        http.Error(w, "failed to fetch updated user", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(updated)
}
```

**Dingo (15 lines):**

```go
func HandleUserUpdate(w: http.ResponseWriter, r: http.Request) {
    let result = processUpdate(r)

    match result {
        Ok(user) => json.NewEncoder(w).Encode(user),
        Err(ApiError.BadRequest(msg)) => http.Error(w, msg, 400),
        Err(ApiError.NotFound(msg)) => http.Error(w, msg, 404),
        Err(ApiError.Internal(msg)) => http.Error(w, msg, 500),
    }
}

func processUpdate(r: http.Request) -> Result<User, ApiError> {
    let userID = r.URL.Query().Get("id").filter(|s| !s.isEmpty()).okOr(ApiError.BadRequest("missing user ID"))?
    let updateReq = json.NewDecoder(r.Body).Decode::<UpdateRequest>().mapErr(|_| ApiError.BadRequest("invalid JSON"))?
    let user = db.GetUser(userID)?.okOr(ApiError.NotFound("user not found"))?

    validateUpdate(updateReq)?
    db.UpdateUser(user.id, updateReq)?
    db.GetUser(user.id)
}
```

### Data Processing Pipeline

**Go (complex nested loops and error handling):**

```go
func ProcessDataBatch(items []Item) ([]ProcessedItem, error) {
    var processed []ProcessedItem

    for _, item := range items {
        if item.IsValid() {
            enriched, err := enrichItem(item)
            if err != nil {
                log.Printf("failed to enrich item %s: %v", item.ID, err)
                continue
            }

            validated, err := validateItem(enriched)
            if err != nil {
                log.Printf("validation failed for item %s: %v", item.ID, err)
                continue
            }

            transformed, err := transformItem(validated)
            if err != nil {
                return nil, fmt.Errorf("transform failed: %w", err)
            }

            processed = append(processed, transformed)
        }
    }

    if len(processed) == 0 {
        return nil, errors.New("no items processed")
    }

    return processed, nil
}
```

**Dingo (functional pipeline):**

```go
func ProcessDataBatch(items: []Item) -> Result<[]ProcessedItem, Error> {
    let processed = items
        .filter { it.isValid() }
        .mapWithLog { enrichItem(it) }
        .mapWithLog { validateItem(it) }
        .map { transformItem(it) }
        .collect()?

    processed.isEmpty()
        ? Err(Error.new("no items processed"))
        : Ok(processed)
}
```

The functional style makes the data flow obvious: filter → enrich → validate → transform → collect.

---

## How does this actually work?

Two-part system, just like TypeScript:

### The Transpiler (`dingo build`)

Takes your `.dingo` files. Parses them. Transforms them to Go AST. Spits out `.go` files and source maps.

The generated Go code looks like what you'd write by hand. Not some machine-generated nightmare.

### The Language Server (`dingo-lsp`)

Wraps gopls (Go's language server). Intercepts LSP requests. Translates positions using source maps. Forwards to gopls.

You get autocomplete, go-to-definition, diagnostics, refactoring — everything gopls does. In VS Code, Neovim, whatever you use.

Your editor thinks it's editing Go. Your terminal thinks it's compiling Go. Only you know you're actually writing Dingo.

---

## What the generated code looks like

Let's see what actually comes out of the transpiler.

**You write this Dingo:**

```go
func findUser(email: string) -> Option<User> {
    let users = db.query("SELECT * FROM users WHERE email = ?", email)
    if users.isEmpty() {
        return None
    }
    return Some(users[0])
}

match findUser("test@example.com") {
    Some(user) => sendEmail(user),
    None => println("Not found")
}
```

**Dingo generates this Go:**

```go
type OptionUser struct {
    value *User
    isSet bool
}

func findUser(email string) OptionUser {
    users := db.query("SELECT * FROM users WHERE email = ?", email)
    if users.isEmpty() {
        return OptionUser{isSet: false}
    }
    return OptionUser{value: &users[0], isSet: true}
}

__opt := findUser("test@example.com")
if __opt.isSet {
    user := *__opt.value
    sendEmail(user)
} else {
    fmt.Println("Not found")
}
```

Clean. Readable. Exactly what you'd write if you were implementing Option types by hand in Go.

No magic. No runtime library. Just structs and if statements.

---

## Your questions, answered

### Is this ready to use right now?

No. We're in Phase 1 of development.

The research is done. The architecture is designed. The features are planned. Now we're building.

Want to follow along? Star the repo. We'll make noise when it's ready.

### Will my existing Go code work with this?

Yes. 100%.

Import any Go package into Dingo. Call any Go function. Use any Go library. It all just works.

You can even mix `.go` and `.dingo` files in the same project. Migrate gradually. No big-bang rewrite required.

### What about performance?

Dingo compiles to Go. Go compiles to machine code. Same machine code.

Zero runtime overhead. No wrapper library. No reflection tricks. No performance penalty.

If your Go code runs fast, your Dingo code runs at exactly the same speed. Because it *is* Go code.

### Why not just wait for Go to add these features?

The Go team has been saying no to sum types for 15 years. They're not changing their minds.

And honestly? They're probably right *for Go*. These features add complexity. Go values simplicity.

But Dingo isn't Go. We're a meta-language. We can add features that transpile to simple Go underneath.

Best of both worlds.

### How is this different from Borgo?

Borgo is awesome. They proved you can transpile Rust-like syntax to Go.

The difference:
- **Borgo** = Rust syntax → Go (for Rust developers who need Go's runtime)
- **Dingo** = Go syntax + modern features (for Go developers who want better ergonomics)

If you love Rust, use Borgo. If you love Go but wish it had sum types, use Dingo.

---

## Roadmap (the realistic version)

### Phase 1: Core Transpiler — 8-10 weeks
- ✅ Research complete
- 🔨 Building sum types right now
- 🔜 Result and Option types
- 🔜 Pattern matching
- 🔜 The `?` operator

**Goal:** Ship something you can actually use to solve Go's error handling problem.

### Phase 2: Ergonomics — 6-8 weeks
- Null safety operators (`?.`, `??`)
- Lambda functions (all styles)
- Map/filter/reduce
- Tuples
- Type-safe enums

**Goal:** Make writing Dingo feel modern without being weird.

### Phase 3: Advanced Features — 4-6 weeks
- Immutability
- Ternary operator
- Pattern guards
- Advanced destructuring

**Goal:** Feature parity with Swift/Kotlin for type safety.

### Phase 4: Language Server — 8-10 weeks
- gopls proxy working
- Source maps translating perfectly
- Full IDE integration

**Goal:** Developer experience that matches or beats regular Go.

### Phase 5: Polish & Ship — 4-6 weeks
- Documentation you'll actually read
- Examples that don't suck
- Real-world testing
- Community feedback

**Target:** v1.0 in about **12-15 months**.

---

## What Dingo adds (and what it doesn't)

### We're adding:
✅ Type safety that prevents bugs at compile time
✅ Modern ergonomics that reduce boilerplate
✅ Features that are proven in Rust/Swift/Kotlin
✅ Zero-cost abstractions (transpiles to clean Go)

### We're NOT adding:
❌ Runtime dependencies (it's just Go underneath)
❌ Magic behavior (no operator overloading, fight me)
❌ Ecosystem fragmentation (works with all Go packages)
❌ Unnecessary complexity (if Go's way is fine, we keep it)

---

## The inspiration wall

Dingo exists because these languages proved it's possible:

**TypeScript** — You can add type safety to an existing language without breaking the world.

**Rust** — Result, Option, pattern matching, and `?` are *chef's kiss* perfect. We're copying all of it shamelessly.

**Swift** — Optional types and safe navigation make nil pointer bugs basically extinct.

**Kotlin** — Pragmatic language design. Features that actually help, not just features for features' sake.

**Borgo** — You can transpile modern syntax to Go and have it work. They proved it.

**templ** — The gopls proxy pattern works. We're using their playbook.

---

## Can I help?

Yes. Here's how:

🌟 **Star the repo** — Shows us people actually want this

💡 **Open issues** — Got ideas? Complaints? Weird edge cases? Tell us.

📖 **Improve docs** — If something's confusing, it's our fault. Help us fix it.

🔨 **Write code** — Check issues tagged "good first issue"

Before you write code, open an issue first. Let's chat about the approach. Saves everyone time.

---

## Project structure (for the curious)

```
dingo/
├── cmd/               # CLI tools (dingo build, dingo-lsp)
├── internal/          # The actual transpiler
│   ├── parser/        # Dingo → AST
│   ├── typechecker/   # Make sure your types make sense
│   ├── transpiler/    # AST → Go code
│   └── lsp/           # Language server magic
├── features/          # Feature proposals (read INDEX.md)
├── docs/              # Documentation
└── examples/          # Example code
```

**Important files:**
- `features/INDEX.md` — Every planned feature with priorities and complexity
- `CLAUDE.md` — Project context (yes, we use AI for help)
- `ai-docs/` — Research notes and architecture decisions

---

## Current status

**Where we are:** Phase 0 → Phase 1 transition

✅ Research done
🔨 Sum types implementation in progress
🔜 Result/Option types next
🔜 Pattern matching after that

**Timeline:** MVP in about **8-10 weeks** if everything goes smoothly.

(It never goes smoothly, so add 30%. That's project management.)

---

## One more thing

Go is an amazing language. This isn't a "Go sucks" project.

But languages evolve. JavaScript got classes. C++ got lambdas. Even Python added type hints.

Go's evolving too — they added generics in 1.18.

Dingo is just evolution they won't add themselves. Sum types. Pattern matching. Result types. The stuff the community keeps asking for.

We're not changing Go. We're building on top of it.

And maybe, just maybe, making your codebase a little bit nicer to work with.

---

<p align="center">
  <strong>Built with 🦴 by developers who love Go but also love sum types.</strong>
</p>

<p align="center">
  <em>Dingo: Because you can love Go and still want better error handling.</em>
</p>

---

**License:** TBD (probably MIT or Apache 2.0)

**GitHub:** [github.com/MadAppGang/dingo](https://github.com/MadAppGang/dingo)

**Community:** GitHub issues · Discussions coming soon

**Status:** In development · Not ready for production · Star to follow progress
