# Unsample Demo Recording
# Usage: vhs docs/demo.tape

Output docs/assets/demo.gif

Set FontSize 16
Set Width 800
Set Height 400
Set Theme "Catppuccin Mocha"
Set Padding 20
Set Shell "zsh"

# Add Go bin to PATH so unsample is found
Env PATH "/Users/lehanouyang/go/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"
Env UNSAMPLE_SECRET "demo-secret-do-not-use-in-production"

# Show the command being typed
Type "unsample debug 'http://localhost:8080/checkout?user=666'"
Sleep 500ms
Enter
Sleep 15s
