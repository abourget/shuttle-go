Linux driver for Contour Design Shuttle Pro V2
==============================================

My goal is to set it up for the Lightworks Non-Linear Editor.

This program supports having modifiers for your Shuttle Pro V2
buttons. Avoid Lightworks key bindings with modifiers however. Capital
letters are great as they cannot be combined, and are more direct and
they are less likely to conflict with your other bindings and
Lightworks recognizes them. All key names used here will work:
http://www.tcl.tk/man/tcl8.4/TkCmd/keysyms.htm

Right now, you need to install `xdotool` from your package
repositories. Eventually, we'll get rid of this dependency.

Buttons layout on the Contour Design Shuttle Pro v2:


           F1   F2   F3   F4

        F5   F6   F7   F8   F9


                (Shuttle)
        S-7 .. S-1  S0  S1 .. S7

     M1        JogL    JogR        M2



              B2        B3
            B1            B4

You can also use `SlowJogL` and `SlowJogR`, to use Frame nudge for example.

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

TODO
----

* Don't require `xdotool`
  * Use xgb's `xtest` package and send the FakeInput directly there.. should work
    a lot better.
  * Document in here all the keys that are work and their proper syntax. Add a few helpers.

* Watch the configuration file, and reload on change.

* Check udev, DISPLAY=:0.0 to start ?
  * Retry ? Check the error message going out.
