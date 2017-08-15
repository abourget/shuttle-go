Linux driver for Contour Design Shuttle Pro V2
==============================================

The goal of this project is to use the Shuttle Pro V2 with the
Lightworks Non-Linear Video Editor.  I'm using v14.

This program supports having **modifiers** for your Shuttle Pro V2
buttons.  So you can multiple the functionality of your buttons.  For
example, you can have different bindings for
<kbd>B1</kbd>+<kbd>F1</kbd> and <kbd>F1</kbd>.

Avoid Lightworks key bindings with modifiers however. Capital
letters are great as they cannot be combined, and are more direct and
they are less likely to conflict with your other bindings and
Lightworks recognizes them.

The key names to use in the X11 bindings are found here:
https://www.cl.cam.ac.uk/~mgk25/ucs/keysymdef.h or you can view them
locally in `/usr/include/X11/keysymdef.h` (stripped of the `XK_`
prefix).

You need to install the `xdotool` package before using this program.

Buttons layout on the Contour Design Shuttle Pro v2:


           F1   F2   F3   F4

        F5   F6   F7   F8   F9


                (Shuttle)
        S-7 .. S-1  S0  S1 .. S7

     M1        JogL    JogR        M2



              B2        B3
            B1            B4


## Slow Jog

In addition to `JogL` and `JogR`, you can define bindings for
`SlowJogL` and `SlowJogR`. For example, you can use a slow jog use to
nudge by one frame at a time.

If you wish to not use slow jog, set the `slow_jog` key to `0` in the
configuration for this app. Otherwise, `slow_jog` represents the
minimum number of milliseconds between two events to be considered
slow. It defaults to 200 ms.


## Run

With:

    sudo shuttle-go /dev/input/by-id/usb-Contour_Design_ShuttlePRO_v2-event-if00


## Install in `udev` with:

**As root**, write file `/etc/udev/rules.d/01-shuttle-go.rules` with contents:

    ACTION=="add", ATTRS{name}=="Contour Design ShuttlePRO v2", MODE="0644"
    ACTION=="remove", ATTRS{name}=="Contour Design ShuttlePRO v2", RUN+="/usr/bin/pkill shuttle-go"

Then run, as **root**:

    udevadm control --reload-rules && udevadm trigger

From that point on, plug in the device, and run `shuttle-go` in any terminal (provided `shuttle-go` is in your `$PATH`).


## License

MIT

## TODO

* Don't require `xdotool`
  * Use xgb's `xtest` package and send the FakeInput directly there..

* Document in here all the keys that are work and their proper syntax. Add a few helpers.

* Watch the configuration file, and reload on change.

* Have a default SlowJog configuration.

* Make it auto-run on plug, with `udev` rules like:

```
    ACTION=="add", ATTRS{name}=="Contour Design ShuttlePRO v2", ENV{MINOR}=="79", RUN+="/home/abourget/go/src/github.com/abourget/shuttle-go/udev-start.sh"
    ACTION=="remove", ATTRS{name}=="Contour Design ShuttlePRO v2", RUN+="/usr/bin/pkill shuttle-go"
```
