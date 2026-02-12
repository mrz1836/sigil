# Sigil Demo

Output demo.gif

Require sigil

# ═══════════════════════════════════════════════════════════════════
# VISUAL SETTINGS
# ═══════════════════════════════════════════════════════════════════

Set Shell "zsh"
Set FontSize 15
Set Width 1100
Set Height 850
Set Padding 20
Set BorderRadius 10
Set WindowBar Colorful
Set TypingSpeed 50ms
Set LineHeight 1.2

# ═══════════════════════════════════════════════════════════════════
# SIGIL THEME - Dark mystical vibes
# ═══════════════════════════════════════════════════════════════════

Set Theme {
  "name": "SigilMystic",
  "black": "#0a0a0f",
  "red": "#ff5f87",
  "green": "#7dd3fc",
  "yellow": "#fcd34d",
  "blue": "#818cf8",
  "magenta": "#c084fc",
  "cyan": "#22d3ee",
  "white": "#e2e8f0",
  "brightBlack": "#475569",
  "brightRed": "#ff6e9c",
  "brightGreen": "#93c5fd",
  "brightYellow": "#fde68a",
  "brightBlue": "#a5b4fc",
  "brightMagenta": "#d8b4fe",
  "brightCyan": "#67e8f9",
  "brightWhite": "#f8fafc",
  "background": "#0a0a0f",
  "foreground": "#c084fc",
  "selection": "#c084fc55",
  "cursor": "#c084fc"
}

# ═══════════════════════════════════════════════════════════════════
# DEMO SCRIPT
# ═══════════════════════════════════════════════════════════════════

# Scene 1: Version check
Sleep 500ms
Type "sigil --version"
Sleep 300ms
Enter
Sleep 1.5s

Hide
Type "clear"
Enter
Show

# Scene 2: Help overview
Type "sigil --help"
Sleep 300ms
Enter
Sleep 3s

Hide
Type "clear"
Enter
Show

# Scene 3: List wallets
Type "sigil wallet list"
Sleep 300ms
Enter
Sleep 2s

Hide
Type "clear"
Enter
Show

# Scene 4: Show receiving address with QR code (the showstopper!)
Type "sigil receive --wallet demo --chain bsv --qr"
Sleep 300ms
Enter
Sleep 500ms
# Enter password (min 8 chars)
#Type "demodemo123"
#Enter
Sleep 4s

Hide
Type "clear"
Enter
Show

# Scene 5: Check balance
Type "sigil balance show --wallet demo"
Sleep 300ms
Enter
Sleep 500ms
# Enter password
#Type "demodemo123"
#Enter
Sleep 3s
