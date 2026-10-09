Output assets/demo.gif

Set Width 1200
Set Height 700
Set FontSize 22
Set Shell bash
Set Padding 48
Set TypingSpeed 25ms
Set Theme {
    "background": "#0a0c0d",
    "foreground": "#fdf9ee",
    "cursor": "#dae656",
    "cursor_text": "#0a0c0d",
    "selection_background": "#383c1d",
    "selection_text": "#fdf9ee",
    "black": "#0a0c0d",
    "red": "#e06c75",
    "green": "#9ae76a",
    "yellow": "#f6e1ac",
    "blue": "#b0b9a8",
    "magenta": "#dae656",
    "cyan": "#9ae76a",
    "white": "#fdf9ee",
    "bright_black": "#383c1d",
    "bright_white": "#fdf9ee"
}

# Build once off-screen so the recording shows results, not compile time.
Hide
Type "go build -ldflags '-X main.version=0.1.0' -o /tmp/cp-demo ./cmd/chatport" Enter
Wait@60s
Show

Type "alias chatport=/tmp/cp-demo" Enter
Sleep 300ms

Type "chatport --version" Enter
Sleep 900ms

Type "chatport parse examples/sample-chatgpt-export.zip" Enter
Sleep 1600ms

Type "chatport parse examples/sample-chatgpt-export.zip --stats" Enter
Sleep 2600ms

Type "chatport parse examples/sample-chatgpt-export.zip --json | head -14" Enter
Sleep 2000ms

Sleep 2500ms
