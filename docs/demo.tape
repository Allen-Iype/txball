# Records docs/demo.gif with VHS (https://github.com/charmbracelet/vhs).
# Run from the repository root:  vhs docs/demo.tape
#
# The game plays itself (-demo), since VHS can't move a mouse. The high
# score goes to a throwaway directory so that yours isn't touched.

Output docs/demo.gif

Set Shell bash
Set FontSize 14
Set Width 800
Set Height 500
Set Padding 8
Set Framerate 20

Hide
Type "go build -o /tmp/txball-demo ./cmd/txball && clear && XDG_CONFIG_HOME=$(mktemp -d) /tmp/txball-demo -demo" Enter
Sleep 4s
Show
Sleep 16s
Hide
Type "q"
Sleep 500ms
