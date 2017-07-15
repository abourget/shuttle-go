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


## Disable the native mouse pointer

The Shuttle acts as a mouse when you plug it into Ubuntu. Disable it with:

    $ xinput --list
    "Virtual core pointer"  id=0    [XPointer]
    "Virtual core keyboard" id=1    [XKeyboard]
    "Keyboard2"     id=2    [XExtensionKeyboard]
    "Mouse2"        id=3    [XExtensionKeyboard]

    # Disable with:
    $ xinput disable 2

Ref: https://unix.stackexchange.com/questions/91075/how-to-disable-keyboard


## Run

With:

    sudo shuttle-go /dev/input/by-id/usb-Contour_Design_ShuttlePRO_v2-event-if00


## License

MIT

##TODO

* Don't require `xdotool`
  * Use xgb's `xtest` package and send the FakeInput directly there.. should work
    a lot better.
  * Document in here all the keys that are work and their proper syntax. Add a few helpers.

* Watch the configuration file, and reload on change.

* Check udev, DISPLAY=:0.0 to start ?
  * Retry ? Check the error message going out.

* Have a default SlowJog configuration.
