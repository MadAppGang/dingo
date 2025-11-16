# LinkedIn Announcement Post

---

I just typed `if err != nil` for the 47th time in a single file.

That's when I decided: enough is enough.

## Here's the thing about Go...

We love it. The simplicity. The performance. The tooling. The deployment story.

But let's be honest for a second.

You're tired of writing the same error handling boilerplate over and over. You've shipped nil pointer panics to production. You've explained to a Rust developer why Go doesn't have sum types and watched them look at you like you just said "we don't believe in seatbelts."

The Go community has been requesting these features for **15 years**. Sum types? 996+ upvotes on the proposal. Type-safe enums? 900+ upvotes. The `?` operator? Every Go developer who's touched Rust wants it.

The Go team keeps saying no. And honestly? They're not wrong. These features add complexity. Go values simplicity.

But what if we didn't have to choose?

## Introducing Dingo

Think TypeScript, but for Go.

A language that compiles to clean, idiomatic Go code. Not a new runtime. Not a new ecosystem. Just better syntax that becomes regular Go.

**Here's what 85 lines of Go looks like in Dingo:**

```go
// Dingo (28 lines)
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

**67% less code. Same safety. Infinitely more readable.**

Look what just happened:
- ✅ The `?` operator eliminated 12 `if err != nil` blocks
- ✅ Lambda functions turned verbose loops into one-liners
- ✅ Optional chaining `?.` replaced nested nil checks
- ✅ Ternary operator cleaned up conditionals
- ✅ Functional utilities made collection operations obvious

The business logic literally jumps off the screen now.

## Is this actually possible?

Yes. [Borgo](https://github.com/borgo-lang/borgo) (4.5k stars) already proved you can transpile to Go successfully. Real production users. Zero runtime overhead.

Dingo builds on Borgo's shoulders with:
- **Full gopls integration** — Your IDE actually works (autocomplete, refactoring, diagnostics)
- **Source maps** — Error messages point to your .dingo files, not generated Go
- **Pure Go implementation** — No Rust toolchain required
- **Multiple lambda styles** — Rust pipes, TS/JS arrows, Kotlin braces, Swift shortcuts
- **Active development** — Not abandoned

## What you get

**Result types:**
```go
func fetchUser(id: string) -> Result<User, Error> {
    let user = db.query(id)?
    return Ok(user)
}
```

**Pattern matching:**
```go
match response {
    Ok(data) => processSuccess(data),
    Err(NotFound) => handle404(),
    Err(ServerError{code, msg}) => logError(code, msg)
}
```

**Null safety:**
```go
let city = user?.address?.city?.name ?? "Unknown"
```

**Lambda functions (pick your style):**
```go
users.filter(|u| u.age > 18)     // Rust
users.filter(u => u.age > 18)    // TypeScript
users.filter { it.age > 18 }     // Kotlin
users.filter { $0.age > 18 }     // Swift
```

All of this transpiles to clean, hand-written-quality Go code. Zero runtime overhead. Works with all Go packages and tools.

## The honest truth

Dingo is for Go developers who love Go but are tired of boilerplate.

Not trying to replace Go. Not forking the language. Just building on top of it.

TypeScript didn't replace JavaScript—it enhanced it. Same playbook.

## Current status

Phase 0 → Phase 1 transition. Research complete. Implementation starting.

MVP expected in 8-10 weeks (or 10-13 weeks if we're being realistic about project timelines).

**This is an early announcement.** Not production-ready. But the vision is clear and the architecture is sound.

## I want your input

Three questions:

**1. What's your #1 pain point with Go right now?**

**2. Which feature would make you actually try Dingo?**
   - Result types + `?` operator
   - Pattern matching
   - Null safety operators
   - Lambda functions
   - Sum types
   - Something else?

**3. Would you use this in production if it had full IDE support and source maps?**

Seriously. Tell me. This project exists to solve real problems for real developers.

If there's a feature you desperately need, or a concern I haven't addressed, drop it in the comments.

## Get involved

⭐ Star the repo: [github.com/MadAppGang/dingo](https://github.com/MadAppGang/dingo)

📖 Read the full feature docs: 12+ planned features with priorities and examples

💬 Join the discussion: What should we build first?

---

Go is an amazing language. This isn't about fixing what's broken.

It's about adding what we've been asking for.

Result types. Pattern matching. Null safety. The stuff the community has wanted for 15 years.

Borgo proved it's possible. Dingo is making it better.

Let's build something developers actually want to use.

**What do you think? Am I solving a real problem or chasing a ghost?**

Drop your thoughts below. Brutal honesty welcome. 👇

---

#golang #programming #opensource #typescript #rust #softwaredevelopment #coding #developertools
