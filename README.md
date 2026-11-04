# Aladdin

[Русский](README.ru.md)

A native Go desktop port of the supplied **Aladdin (USA, Final Cut)** for Windows, Linux and macOS. Game logic runs in Go; converted graphics and game data are embedded. The runtime uses no ROM, console/CPU emulator or external game process.

Main menu/settings/Sound Test controls, native gameplay across converted scenes, Genie/Abu bonuses, Continue, Game Over, record handling, the new-game scarab animation and controls card, post-story level-title screens, automatic attract playback, the final carpet flight/Aladdin-and-Jasmine animation/end credits and native story pages for nine stage transitions are implemented. Exact sound-driver channel behavior, publisher/book introductions and remaining gameplay resource-loading transitions, complete attract timing/visual equivalence, some interactions/objectives and full unmodified progression remain unfinished or unverified. Loading and input/raster timing outside the qualified recordings are not established. A small late Rug Ride background discrepancy remains.

Story scenes use the original portraits, backgrounds, font, text blocks, pauses, palette fades and animated actors. Nine stage routes include the original black resource-transfer intervals and scrolling, with source-based keyboard sampling during loading. Recorded story comparisons cover every video boundary from the first story page to the end of the story; publisher boot logos, publisher/book introductions and the other gameplay resource loaders remain unfinished or unverified. Starting a new game runs the original scarab animation and controls card, including their waits, palette changes, skip rules and music handoff. The following level-title screen uses the original sprite lettering, wave-scrolled background, palette fades, button polling and sound requests. Its Agrabah recording covers 4,405 video boundaries in 12 input/audio-settings cases; a separate test follows normal menu Start through the entire opening and story into this screen. All 14 stage routes are checked for entry/bypass and inventory preservation; other title texts have not received independent original-frame comparisons. Agrabah Market now follows the original resource-load waits, temporary actor preparation, random-state progression and fade into play. Eight independent normal-input/skip recordings cover 1,994 loading/gameplay video boundaries, including controls held during loading and the first 121 ordinary updates. Desert and Agrabah Rooftops also use their original captions and stage-specific loading/preparation waits. Their isolated loader comparisons cover 3,990 video boundaries in 16 input/skip cases; each reference stage is selected at loader entry. Natural level-to-level progression, other stage loaders and further difficulty/audio combinations still need equivalent validation. Direct `-stage` and scene-selector launches bypass these introductions for development.

The terminal campaign route now runs the original carpet flight, Aladdin/Jasmine animation and end credits before returning to the menu. Four controlled terminal-route recordings compare all 13,386 video boundaries, including held actions, credits exit and disabled music. Actor state is compared outside black resource jobs, which run atomically in Go. These recordings select the terminal route explicitly; they do not prove a natural boss win or a complete playthrough. A separate integration test follows both campaign branches, including all optional Abu rooms, with explicitly supplied exit flags and checks inventory/settings transfer through the ending. Publisher/book loading on the return to the main menu remains unqualified.

Attract mode plays the five original recorded input streams, followed by the original scrolling credits. Four demonstrations match complete recordings; one retains a documented video-timing difference. X, C, Space/Z or Enter returns from a demonstration. Enter skips credits. C preserves the original text-stall behavior in attract credits. End-of-game credits ignore action buttons; Enter exits them.

The record is retained across levels, continues and new games during the process. The supplied Final Cut build disables its record-message routine while retaining a black-screen wait; this behavior is preserved. All 94 Sound Test entries are connected. Music and effects are synthesized at run time: a native Go port of the original GEMS sound driver logic plays the game's own sequences, instrument patches, pitch envelopes and DAC samples (`gems.bin`, 215 KB, converted unchanged from ROM 1E54F4-219D50) through Go models of the YM2612 FM and SN76489 PSG chips. Tempo, voice allocation and stealing, priorities, PSG envelopes, pitch envelopes, DAC streaming (including its pacing of driver work) are reproduced; the driver state was compared byte by byte with the original driver's memory at every video boundary. No recordings, ROM image, CPU or Z80 program are used. See CHANGES.ru.md for the measured accuracy.

![Agrabah](screenshots/agrabah.png)
![Guard combat](screenshots/combat.png)

## Controls

| Keys | Action |
|---|---|
| Arrows or WASD | Move; navigate menus; climb ropes and traverse hanging rails |
| Space or Z | Jump; confirm the main-menu selection |
| X | Sword; Abu swipe; confirm menu selection |
| C | Throw an apple; confirm menu selection |
| Enter | Confirm main menu; leave settings |
| X, C, Space or Z in settings | Change the selected setting |
| Up/Down in Sound Test | Select a sound, with the original held-button repeat |
| C in Sound Test | Play the selected sound |
| X, Space/Z or Enter in Sound Test | Return to settings |
| Left/A or Right/D in Continue | Decline / continue; action buttons do not answer |
| Space/Z, X or C in Genie bonus | Claim when ready; release to resume spinning; Enter does not claim |
| Space/Z, X, C or Enter during interludes | Skip when the original scene permits it |
| Up/Down during Rug Ride | Steer the carpet vertically |
| Up near the merchant | Buy a life on his left (5 gems), or a continue on his right (10 gems) |
| Esc | Return to the title menu; exit from the title menu |
| F11 | Full screen |

### Gamepad

Xbox (XInput), PlayStation DualSense/DualShock, Switch Pro and other controllers known to the SDL controller database work on Windows, Linux and macOS, over USB or Bluetooth. They can be connected at any time and work alongside the keyboard.

| Button | Action | Keyboard |
|---|---|---|
| D-pad or left stick | Move, navigate menus | Arrows/WASD |
| A / Cross | Jump | Space/Z |
| X / Square | Sword | X |
| B / Circle, Y / Triangle | Throw an apple | C |
| Menu / Options | Start | Enter |
| View / Share (Create) | Return to the title menu; exit from the title menu | Esc |

On Linux the controller is read from `/dev/input/event*`; desktop distributions grant this to the logged-in user. If a pad is not detected, check that your user can read those devices.

In Abu rooms, Space/Z jumps and X swipes; C and vertical directions have no action. Tracking settings use the original Default/Classic/Tight camera behavior and persist across level changes and continues. Music and effects have separate playback settings. Re-enabling music in settings restarts the title song. `-mute` disables the audio device entirely.

A normal launch starts like the console at power-on: the Disney Software/Virgin Games co-production intro (Genie flight, title theme from boundary 69, A/B/C/Start skip window 90–369), then the title menu with the intro's original frame counter and random seed.

Development controls exist only with `-dev` (or another development launch option): **F2 on the title menu** opens the scene selector; in gameplay F2 toggles the free camera. **F1** shows contours/markers, **F3** opens the sprite viewer, and **R** resets the scene. In the viewer arrows change frames and Space toggles browsing.

## Build and run

Use **Go 1.25 or newer**. Ebitengine **2.10.5** is pinned. The first build needs network access to download Go modules. `main.go` is at the project root.

```sh
go mod download
go build -trimpath -o Aladdin .
./Aladdin
```

Windows:

```sh
go build -trimpath -ldflags="-H windowsgui" -o Aladdin.exe .
```

All six desktop targets support `CGO_ENABLED=0`: `GOOS=windows`, `linux` or `darwin`, with `GOARCH=amd64` or `arm64`. Example cross-builds from a Unix shell:

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags="-H windowsgui" -o Aladdin.exe .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o Aladdin-linux .
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -o Aladdin-macos .
```

No C compiler or SDK is required for these builds. Linux needs X11/XWayland and an OpenGL runtime; on Debian/Ubuntu:

```sh
sudo apt install libx11-6 libgl1 libglx-mesa0 libxcursor1 libxi6 libxinerama1 libxrandr2 libxrender1 libxext6 libasound2
```

The window scales the original 320×224 image without smoothing. PNG/ICO/ICNS and editable SVG icons are included; the PNG is embedded as the window icon. The ICNS can be used in a macOS `.app` bundle.

Development launches: `./Aladdin -dev`, `-scenes`, `-stage 1`, `-genie 10`, or `-stage 1 -last-life 0`. `-frames 120 -capture scene.png` saves the native renderer output and exits. `-last-life 3` exercises Continue after a normal last-life death. These are development probes, not normal wins.

Verification command: `go vet ./...`. The automated comparison suite and original reference recordings are not part of this release archive. See `CHANGES.ru.md` for the measured differences from the original.

## Credits and licensing

Original Aladdin/Final Cut graphics, audio and game data belong to their respective rights holders and are not covered by the new-code license. This project grants no rights to redistribute original assets or trademarks.

The newly written Go code and lamp icon use the MIT license:

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
