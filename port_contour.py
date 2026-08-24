#!/usr/bin/env python3
"""Port the Contour Design shuttle config (contour_default_backup.json) into
shuttle-go's config format: one JSON file per app, each matching on window
title. Reads the source JSON, groups actions by AppContextConfig, maps the
Contour controls to shuttle-go keys, and writes one file per app.
"""
import json
import os
import re
import sys

SRC = "contour_default_backup.json"
OUT_DIR = "examples"

# --- Contour control -> shuttle-go binding key ------------------------------
# The ShuttlePRO v2 has 15 buttons, 2 dials, and a 15-position shuttle wheel.
# Contour's "DeviceButton_N" is the Nth button (1-based).
def control_key(mb: str) -> str:
    m = re.fullmatch(r"DeviceButton_(\d+)", mb)
    if m:
        n = int(m.group(1))
        if n <= 9:
            return f"F{n}"
        if n == 10:
            return "B1"
        if n == 11:
            return "B2"
        if n == 12:
            return "B3"
        if n == 13:
            return "B4"
        if n == 14:
            return "M1"
        if n == 15:
            return "M2"
        return ""
    if mb == "DeviceDialLeft":
        return "JogL"
    if mb == "DeviceDialRight":
        return "JogR"
    m = re.fullmatch(r"DeviceJogWheelZone(-?\d+)", mb)
    if m:
        return f"S{m.group(1)}"
    return ""  # transitions and anything else: not a stable position

# --- Contour KeyCode -> shuttle-go key string --------------------------------
# Letters/digits map to themselves (uppercased). Shifted symbols map to
# "Shift+<base key>" (the base key that produces the character). A few common
# symbols get a named alias (see NAMED below) so the config reads nicely.
KEYMAP = {
    "KEY_SPACE": "space",
    "KEY_0": "0", "KEY_1": "1", "KEY_2": "2", "KEY_3": "3", "KEY_4": "4",
    "KEY_5": "5", "KEY_6": "6", "KEY_7": "7", "KEY_8": "8", "KEY_9": "9",
    "KEY_A": "A", "KEY_B": "B", "KEY_C": "C", "KEY_D": "D", "KEY_E": "E",
    "KEY_F": "F", "KEY_G": "G", "KEY_H": "H", "KEY_I": "I", "KEY_J": "J",
    "KEY_K": "K", "KEY_L": "L", "KEY_M": "M", "KEY_N": "N", "KEY_O": "O",
    "KEY_P": "P", "KEY_Q": "Q", "KEY_R": "R", "KEY_S": "S", "KEY_T": "T",
    "KEY_U": "U", "KEY_V": "V", "KEY_W": "W", "KEY_X": "X", "KEY_Y": "Y",
    "KEY_Z": "Z",
    # Shifted symbols: emit the literal symbol as the key with Shift held,
    # e.g. "Shift+'". The symbol itself is a valid key in the keyboardKeys map.
    "KEY_!": "Shift+!", "KEY_#": "Shift+#", "KEY_$": "Shift+$",
    "KEY_%": "Shift+%", "KEY_&": "Shift+&", "KEY_'": "Shift+'",
    "KEY_(": "Shift+(", "KEY_.": "Shift+.", "KEY_`": "Shift+`",
    "KEY_F3": "F3", "KEY_F4": "F4", "KEY_F5": "F5", "KEY_F7": "F7",
    "KEY_F8": "F8", "KEY_F11": "F11", "KEY_F12": "F12",
}


def key_string(kc: str) -> str:
    if kc in KEYMAP:
        return KEYMAP[kc]
    # Fallback: KEY_<X> where <X> is a single letter.
    m = re.fullmatch(r"KEY_([A-Z])", kc)
    if m:
        return m.group(1)
    return ""


def is_shuttle_position(key: str) -> bool:
    """True for the held shuttle-wheel positions (S-7..S-1, S1..S7)."""
    m = re.fullmatch(r"S(-?)(\d+)", key)
    if not m:
        return False
    return 1 <= int(m.group(2)) <= 7


def build_value(action: dict, key: str = "") -> object:
    """Return the shuttle-go binding value (string or object) for an action.

    The value is a plain key string in the common case. It becomes an object
    when the action either repeats more than once or carries a human comment
    (the Contour "Comment"), so that the comment is preserved via the object
    form's "comment" field instead of being lost.

    For a held shuttle position, a Contour "repeat N over DelayRepeat ms" is
    re-expressed as a single repeat *interval* (delay_ms = DelayRepeat / N),
    because shuttle-go now repeats a held position at a fixed rate rather than
    firing a fixed number of taps. Button bindings keep the plain "repeat" count.
    """
    kc = action.get("KeyCode", "")
    if not kc:
        return ""
    base = key_string(kc)
    if not base:
        return ""
    parts = []
    if action.get("UseCtrl"):
        parts.append("Ctrl")
    if action.get("UseAlt"):
        parts.append("Alt")
    if action.get("UseShift"):
        parts.append("Shift")
    out_key = "+".join(parts + [base])
    repeat = action.get("Repeat", 0) or 0
    delay = action.get("DelayRepeat", 0) or 0
    comment = action.get("Comment", "").strip()
    if repeat > 1 or comment:
        obj = {"key": out_key}
        if repeat > 1:
            if is_shuttle_position(key):
                # Convert "repeat N over delay ms" into a repeat interval.
                interval = round(delay / repeat) if delay > 0 else 25
                obj["delay_ms"] = interval
            else:
                obj["repeat"] = repeat
                obj["delay_ms"] = delay if delay > 0 else 25
        if comment:
            obj["comment"] = comment
        return obj
    return out_key


def port_app(app_name: str, exe: str, actions: list) -> dict:
    """Build one shuttle-go app config from its Contour actions."""
    bindings = {}
    for a in actions:
        if a.get("ActionType") != "keypress":
            continue  # click/scroll/same_as_lower_value are not portable
        k = control_key(a.get("MouseButton", ""))
        if not k:
            continue
        v = build_value(a, k)
        if v == "":
            continue
        if k in bindings:
            # A control is bound twice (e.g. a keypress and a non-keypress on
            # the same button). Keep the first keypress; note the collision.
            continue
        bindings[k] = v

    if not bindings:
        return {}

    # Window-title matcher: the app's display name, plus the bare exe base
    # name (without .exe) so it also matches a Linux window titled e.g. "Resolve".
    # These are plain substrings (no regex metachars in practice), so no
    # re.escape — that would leave an ugly backslash before the space.
    base = os.path.splitext(os.path.basename(exe))[0]
    titles = [app_name]
    if base and base.lower() != app_name.lower():
        titles.append(base)

    return {
        "name": app_name,
        "match_window_titles": titles,
        "slow_jog": 200,
        "bindings": bindings,
    }


def main():
    with open(SRC) as f:
        data = json.load(f)

    # exe per ConfigName
    exe_by_name = {
        c["ConfigName"]: c.get("ProgramContextPath", "")
        for c in data.get("AppContextConfigurations", [])
    }

    # Use the ShuttlePRO v2 profile (the first "default" profile).
    profile = None
    for p in data.get("ProfilesPreferences", []):
        if p.get("DeviceModel") == "shuttleproV2":
            profile = p
            break
    if profile is None:
        profile = data["ProfilesPreferences"][0]

    # Group actions by app.
    by_app = {}
    for a in profile.get("Actions", []):
        by_app.setdefault(a.get("AppContextConfig", ""), []).append(a)

    os.makedirs(OUT_DIR, exist_ok=True)
    written = []
    for app_name in sorted(by_app):
        exe = exe_by_name.get(app_name, "")
        app = port_app(app_name, exe, by_app[app_name])
        if not app:
            print(f"SKIP {app_name!r}: no portable bindings")
            continue
        fname = re.sub(r"[^A-Za-z0-9]+", "_", app_name).strip("_").lower()
        path = os.path.join(OUT_DIR, f"{fname}.json")
        with open(path, "w") as f:
            json.dump({"apps": [app]}, f, indent=4)
            f.write("\n")
        written.append((app_name, path, len(app["bindings"])))
        print(f"WROTE {path}: {len(app['bindings'])} bindings")

    print(f"\n{len(written)} app files written to {OUT_DIR}/")


if __name__ == "__main__":
    main()
