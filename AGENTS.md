# BreadCalc

## Commands

- `go run .` runs the Bubble Tea terminal app; `go run ./cmd/breadcalc-gui` runs the Fyne GUI. The module declares Go 1.21.
- `go test ./...` runs the suite (also the release CI test step). Focus with `go test ./engine -run '^TestGoldenAgainstTestPy$'` or substitute a package and test name.
- GUI tests use Fyne's test app. `BREADCALC_SNAPSHOTS=/path go test ./gui -run '^TestSnapshots$'` writes PNGs there; without the variable the snapshot test skips.
- `./build.sh v1.0.1` cross-builds and archives the **terminal app only**, with `CGO_ENABLED=0`. It deletes and recreates `dist/` first. Release CI runs tests before this script on `v*` tags.

## Structure and pitfalls

- The root package is the terminal UI; `cmd/breadcalc-gui` and `gui/` are the separate Fyne UI. `recipe/` contains the GUI's presets and aggregation; the terminal app has its own recipe code in `calc.go` and `recipes.go`.
- `engine/` is UI-independent and uses the standard library. `engine/default_model.txt` is embedded by `engine/store.go`; change it when changing built-in formulas, and check the numerical reference tests in `engine/engine_test.go`.
- Both apps choose an existing `breadcalc-model.txt` in the **current working directory** before the user config directory's `breadcalc/model.txt`. Loading a missing model can create it; saving makes a `.bak`. Avoid accidentally changing a real user model when running the apps.
- The README describes the terminal app and may not cover the GUI; check code and `build.sh` for current behavior.
