# HTTP Status Code or Custom Status Code? An Elegant Error Handling Solution in Go

When developing REST APIs, have you ever encountered these problems:

- Error messages are disorganized and hard to locate
- Error prompts shown to users are obscure and difficult to understand
- Sensitive information is exposed in production environment
- Error handling code is repetitive and hard to maintain

This article introduces an elegant error handling solution that makes your code clearer, more maintainable, and provides a better user experience.

## Pain Points in Error Handling

In traditional error handling approaches, we often face the following issues:

1. Inconsistent error messages
   - Some use numeric status codes
   - Some use string descriptions
   - Some directly return underlying errors

2. Repetitive error handling code
   - Each interface requires error handling logic
   - Logging is scattered everywhere
   - HTTP status code mapping is confusing

3. Poor user experience
   - Error prompts are not user-friendly
   - Lack of contextual information
   - Difficult to locate problems

## Traditional Error Handling Solutions

Let's first look at traditional error handling approaches:

```go
// Approach 1: Direct error return
func findUser(ctx *gin.Context) {
    user, err := db.FindUser()
    if err != nil {
        ctx.JSON(500, gin.H{"error": err.Error()})
        return
    }
    ctx.JSON(200, user)
}

// Approach 2: Using status codes
func findUser(ctx *gin.Context) {
    user, err := db.FindUser()
    if err != nil {
        ctx.JSON(500, gin.H{
            "code": 1001,
            "msg": "Database query failed"
        })
        return
    }
    ctx.JSON(200, user)
}
```

These approaches, while simple and direct, have some obvious drawbacks:

1. Security issues
   - Approach 1 directly exposes underlying errors to users, potentially leaking sensitive information
   - Database errors may contain internal information like table structure, SQL statements

2. Communication complexity
   - Is status code 400 an HTTP status code or a custom status code?
   - Must deserialize response body to know the status
   - Duplicate definitions, both HTTP status code 200 and custom status code 0 indicate success

3. Poor maintainability
   - Numeric error codes (like 1001) lack semantics and are hard to understand
   - Developers need to consult documentation to understand error meanings
   - Even error code definers may forget their specific meanings

## An Elegant Error Handling Solution

Let's first see how the elegant error handling solution is used:

```go
var ErrBadRequest = reason.NewError("ErrBadRequest", "Invalid request parameters")

func (u *UserAPI) getUser(ctx *gin.Context, _ *struct{}) (*user.UserOutput, error) {
    return u.core.GetUser(in.ID)
}

// package user
func (u *Core) GetUser(id int64) (*UserOutput, error) {
    // Parameter validation
    if err != nil {
        return nil, ErrBadRequest.With(err.Error(), "xx parameter should be between 10~100")
    }
    // Correct processing logic...
}
```

Remember the `web.WarpH` function mentioned in the previous article? Its error response actually calls `web.Fail(err)`, which checks if the error is of type `reason.Error`. If it is, it returns the defined HTTP status code, reason, and msg to the client.

Similar to:

HTTP Status Code: 400 (default for all errors)
```json
{
    "reason": "Error for program recognition",
    "msg": "Error message description for users",
    "details": [
        "Field transmission error",
        "You can fix it this way",
        "Check documentation for more information"
    ]
}
```

### Error Code Design Considerations

Traditional solutions using numeric error codes (like 1001 for database errors) have obvious disadvantages: lack of semantics, requiring documentation consultation, and definers easily forgetting meanings.

Therefore, we use strings as error codes, with clear advantages:

1. Strong self-explanatory nature
   - `ErrBadRequest` is more intuitive than `1001`
   - Error codes serve as documentation
   - Facilitates code review and debugging

2. Good extensibility
   - Can use module prefixes for distinction
   - Avoids error code conflicts
   - Quickly locates problem sources

During frontend integration, when certain interfaces encounter errors, frontend developers might be confused and need to consult backend developers. Some issues might just be parameter problems. What if the error response included solutions? Could this simplify integration complexity?

To prevent users from seeing technical errors or developers lacking contextual information, we divide error information into four attributes:

```go
type Error struct {
    Reason     string
    Msg        string
    Details    []string
    HTTPStatus int
}
```

The role of each field:

1. `reason` field
   - Uses PascalCase English to describe error reasons
   - Used for internal program error type determination
   - Supports error code mapping to HTTP status codes

2. `msg` field
   - Uses developer's native language to describe errors
   - User-oriented, provides friendly prompts

3. `details` field
   - Provides extended error information
   - Developer-oriented, aids debugging
   - Can be hidden in production environment by calling `web.SetRelease()` to avoid leaking sensitive information

4. `HTTPStatus`
   - HTTP response status code
   - Defaults to 400
   - Common status codes 200, 400, 401 are usually sufficient, keeping it simple

## Usage Documentation

1. Using predefined error types
   - `reason.ErrBadRequest`: Request parameter error
   - `reason.ErrStore`: Database error
   - `reason.ErrServer`: Server error

2. Error message handling
   - Use `SetMsg()` method to modify user-friendly prompts
   - Use `Withf()` method to add developer assistance, increasing error context

Adopt a single reason principle, meaning errors with the same reason are considered the same error.

```go
// Are e1 and e2 the same error?
e1 = NewError("e1", "e1")
e2 = NewError("e2", "e1")
```

```go
// e3 and e2 are the same error
e2 := NewError("e2", "e2").SetHTTPStatus(200).With("e2-1")
e3 := fmt.Errorf("e3:%w", e2)
if !errors.Is(e3, e2) {
    t.Fatal("expect e3 is e2, but not")
}
```

```go
// Convert error to *reason.Error struct
var e5 *reason.Error
if !errors.As(e4, &e5) {
    t.Fatal("expect e4 as e5, but not")
}
```

## Summary

Through reasonable layering and encapsulation, we have achieved:

1. Unified error handling process
2. Friendly user prompts
3. Detailed developer information
4. Secure error exposure

This design ensures both development efficiency and user experience. If you're looking for an elegant error handling solution, why not try this approach?

## About goddd

The error handling introduced in this article is a core component of the [goddd](https://github.com/ixugo/goddd) project. goddd is a Go project based on DDD (Domain-Driven Design) principles, providing a series of tools and best practices to help developers build maintainable and extensible applications.

If you're interested in the content introduced in this article, welcome to visit the [goddd project](https://github.com/ixugo/goddd) to learn more. The project provides complete example code and detailed documentation to help you get started quickly.