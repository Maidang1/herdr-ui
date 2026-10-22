package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheDefaultThemeIsTheOneTheRustClientUses(t *testing.T) {
	found := Default()
	if found.Background != 0x101419 {
		t.Fatalf("the background came out as %06x", found.Background)
	}
	if found.Foreground != 0xd8dee9 {
		t.Fatalf("the foreground came out as %06x", found.Foreground)
	}
	if found.Cursor != found.Foreground {
		t.Fatalf("the cursor came out as %06x", found.Cursor)
	}
	if found.Palette[1] != 0x800000 {
		t.Fatalf("the red came out as %06x", found.Palette[1])
	}
	if found.Palette[15] != 0xffffff {
		t.Fatalf("the white came out as %06x", found.Palette[15])
	}
	// The cube of colors and the ramp of grays.
	if found.Palette[16] != 0x000000 {
		t.Fatalf("the first of the cube came out as %06x", found.Palette[16])
	}
	if found.Palette[231] != 0xffffff {
		t.Fatalf("the last of the cube came out as %06x", found.Palette[231])
	}
	if found.Palette[232] != 0x080808 {
		t.Fatalf("the first of the grays came out as %06x", found.Palette[232])
	}
	if found.Palette[255] != 0xeeeeee {
		t.Fatalf("the last of the grays came out as %06x", found.Palette[255])
	}
}

func TestTheChromeIsMixedOutOfTheColors(t *testing.T) {
	found := Default()
	// Five percent of the foreground over the background.
	if want := Mix(found.Background, found.Foreground, 5); found.Surface != want {
		t.Fatalf("the surface came out as %06x, want %06x", found.Surface, want)
	}
	if want := Mix(found.Background, found.Foreground, 12); found.Active != want {
		t.Fatalf("the active color came out as %06x, want %06x", found.Active, want)
	}
	if want := Mix(found.Background, found.Foreground, 55); found.Muted != want {
		t.Fatalf("the muted color came out as %06x, want %06x", found.Muted, want)
	}
}

func TestMixTakesThePercentOfTheColorOverIt(t *testing.T) {
	if got := Mix(0x000000, 0xffffff, 50); got != 0x7f7f7f {
		t.Fatalf("the mix came out as %06x", got)
	}
	if got := Mix(0x000000, 0xffffff, 100); got != 0xffffff {
		t.Fatalf("the mix came out as %06x", got)
	}
	if got := Mix(0x000000, 0xffffff, 0); got != 0x000000 {
		t.Fatalf("the mix came out as %06x", got)
	}
}

func TestAGhosttyThemeFileIsRead(t *testing.T) {
	text := `# A theme file.
background = 1d1f21
foreground = c5c8c6
cursor-color = f0c674
bold-color = #ffffff
palette = 0=#000000
palette = 1=#cc6666
palette = 15=#eeeeec
font = Some Font
`
	found, err := ParseGhostty(text)
	if err != nil {
		t.Fatalf("the theme: %v", err)
	}
	if found.Background != 0x1d1f21 {
		t.Fatalf("the background came out as %06x", found.Background)
	}
	if found.Foreground != 0xc5c8c6 {
		t.Fatalf("the foreground came out as %06x", found.Foreground)
	}
	if found.Cursor != 0xf0c674 {
		t.Fatalf("the cursor came out as %06x", found.Cursor)
	}
	if found.Bold == nil || *found.Bold != 0xffffff {
		t.Fatalf("the bold color came out as %v", found.Bold)
	}
	if found.Palette[1] != 0xcc6666 {
		t.Fatalf("the red came out as %06x", found.Palette[1])
	}
	// The colors the file does not name keep the ones of the default.
	if found.Palette[2] != Default().Palette[2] {
		t.Fatalf("the green came out as %06x", found.Palette[2])
	}
}

func TestTheCursorOfAThemeIsTheForegroundWhenTheFileHasNone(t *testing.T) {
	found, err := ParseGhostty("foreground = aabbcc\n")
	if err != nil {
		t.Fatalf("the theme: %v", err)
	}
	if found.Cursor != found.Foreground {
		t.Fatalf("the cursor came out as %06x", found.Cursor)
	}
}

func TestABrokenThemeFileIsRefused(t *testing.T) {
	if _, err := ParseGhostty("background = notacolor\n"); err == nil {
		t.Fatal("the theme was taken")
	}
	if _, err := ParseGhostty("palette = 300=#ffffff\n"); err == nil {
		t.Fatal("the palette was taken")
	}
	if _, err := ParseGhostty("palette = 1\n"); err == nil {
		t.Fatal("the palette was taken")
	}
}

func TestThePalettesTheRustClientWritesByHandAreThere(t *testing.T) {
	nord, ok := Builtin("Nord")
	if !ok {
		t.Fatal("the palette of Nord is missing")
	}
	if nord.Background != 0x2e3440 || nord.Foreground != 0xd8dee9 {
		t.Fatalf("the palette of Nord came out as %06x and %06x", nord.Background, nord.Foreground)
	}
	dracula, ok := Builtin("Dracula")
	if !ok {
		t.Fatal("the palette of Dracula is missing")
	}
	if dracula.Background != 0x282a36 || dracula.Palette[1] != 0xff5555 {
		t.Fatalf("the palette of Dracula came out as %06x", dracula.Background)
	}
	if _, ok := Builtin("No Such Theme"); ok {
		t.Fatal("a theme that is not there was found")
	}
}

func TestAThemeMayNameOneForEachSideOfTheSystem(t *testing.T) {
	pair := ParsePair("light:Nord,dark:Dracula")
	if pair.Light != "Nord" || pair.Dark != "Dracula" {
		t.Fatalf("the pair came out as %+v", pair)
	}
	pair = ParsePair("dark:Dracula,light:Nord")
	if pair.Light != "Nord" || pair.Dark != "Dracula" {
		t.Fatalf("the pair came out as %+v", pair)
	}
	pair = ParsePair("Nord")
	if pair.Light != "Nord" || pair.Dark != "Nord" {
		t.Fatalf("the pair came out as %+v", pair)
	}
	if !System("light:Nord,dark:Dracula") {
		t.Fatal("the pair is not a pair")
	}
	if System("Nord") {
		t.Fatal("a single theme is a pair")
	}
}

func TestAThemeIsLookedForInTheThemeFolders(t *testing.T) {
	folder := t.TempDir()
	themes := filepath.Join(folder, "themes")
	if err := os.MkdirAll(themes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themes, "Mine"), []byte("background = 101010\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := File("Mine", []string{themes})
	if err != nil {
		t.Fatalf("the theme file: %v", err)
	}
	if path != filepath.Join(themes, "Mine") {
		t.Fatalf("the theme file came out as %q", path)
	}
	if _, err := File("Missing", []string{themes}); err == nil {
		t.Fatal("a theme that is not there was found")
	}
	// A name that is a path is no theme.
	if _, err := File("../escape", []string{themes}); err == nil {
		t.Fatal("a path was taken for a theme")
	}
}

func TestTheNamesOfTheThemesAreListed(t *testing.T) {
	folder := t.TempDir()
	themes := filepath.Join(folder, "themes")
	if err := os.MkdirAll(themes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themes, "Mine"), []byte("background = 101010\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := Available([]string{themes})
	found := false
	for _, name := range names {
		if name == "Mine" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the themes came out as %v", names)
	}
	if names[0] != "Default" {
		t.Fatalf("the themes came out as %v", names)
	}
}

func TestTheThemeOfTheDaemonIsFollowed(t *testing.T) {
	folder := t.TempDir()
	root := filepath.Join(folder, "herdr")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", folder)
	daemon := `
[theme]
name = "nord"
auto_switch = true
light_name = "Catppuccin Latte"
dark_name = "Nord"
`
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte(daemon), 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := Resolve(FollowHerdr, false)
	if err != nil {
		t.Fatalf("the theme: %v", err)
	}
	if found.Background != 0x2e3440 {
		t.Fatalf("the background came out as %06x", found.Background)
	}
	found, err = Resolve(FollowHerdr, true)
	if err != nil {
		t.Fatalf("the theme: %v", err)
	}
	if found.Background != 0xeff1f5 {
		t.Fatalf("the background came out as %06x", found.Background)
	}
}

func TestAThemeFileIsMatchedWithoutRegardToItsCase(t *testing.T) {
	folder := t.TempDir()
	themes := filepath.Join(folder, "themes")
	if err := os.MkdirAll(themes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themes, "Gruvbox Dark"), []byte("background = 282828\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := resolveName("gruvbox", []string{themes})
	if err != nil {
		t.Fatalf("the theme: %v", err)
	}
	if found.Background != 0x282828 {
		t.Fatalf("the background came out as %06x", found.Background)
	}
}
