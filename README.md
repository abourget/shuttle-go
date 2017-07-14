Linux driver for Contour Design Shuttle Pro V2
==============================================

My goal is to set it up for the Lightworks Non-Linear Editor.




Buttons layout on the Contour Design Shuttle Pro v2:


           F1   F2   F3   F4

        F5   F6   F7   F8   F9


                (Shuttle)
        S-7 .. S-1  S0  S1 .. S7

     M1        JogL    JogR        M2



              B2        B3
            B1            B4


See


TODO
----

* Fix up timings, make sure we properly support shortcuts with
  Ctrl+Shift and it doesn't clog the program. Perhaps optimize and
  keep certain keys pressed, until not needed anymore.  Especially
  using the Jog and Shuttle.

* Make sure we have a solution to ignore the device as a generic HID
  under Ubuntu.  We can't have mouse clicks on top of our bindings!

* Check udev, DISPLAY=:0.0 to start ?
  * Retry ? Check the error message going out.

* Try the xdotool with the latest bindings, XTest-based.
  * Use xgb's `xtest` package and send the FakeInput directly there.. should work
    a lot better.


Disable the native mouse pointer provided by the Shuttle with:

    $ xinput --list
    "Virtual core pointer"  id=0    [XPointer]
    "Virtual core keyboard" id=1    [XKeyboard]
    "Keyboard2"     id=2    [XExtensionKeyboard]
    "Mouse2"        id=3    [XExtensionKeyboard]

    # Disable with:
    $ xinput set-int-prop 2 "Device Enabled" 8 0

Ref: https://unix.stackexchange.com/questions/91075/how-to-disable-keyboard
