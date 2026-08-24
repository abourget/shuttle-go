/* extension.js
 *
 * Exposes shell state over the session D-Bus, so other tools (e.g.
 * shuttle-go) can read it on Wayland, where the shell is the only thing that
 * reliably knows it. A single Info method returns the focused window's title
 * and wm_class, plus the current pointer position.
 *
 * Modeled on ickyicky/window-calls: a D-Bus interface is exported on the
 * session bus and its methods are answered by methods of the same object.
 *
 * Usage:
 *   dbus-send --session --print-reply=literal \
 *       --dest=org.gnome.Shell \
 *       /org/gnome/Shell/Extensions/ShuttlePro \
 *       org.gnome.Shell.Extensions.ShuttlePro.Info \
 *     | jq .
 *
 *   ->  {"title":"Lightworks","wm_class":"lightworks","x":100,"y":300}
 *
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import Gio from 'gi://Gio';

const SHUTTLE_PRO_DBUS_IFACE = `
<node>
   <interface name="org.gnome.Shell.Extensions.ShuttlePro">
      <method name="Info">
         <arg type="s" direction="out" name="info" />
      </method>
   </interface>
</node>`;


export default class Extension {
  enable() {
    this._dbus = Gio.DBusExportedObject.wrapJSObject(SHUTTLE_PRO_DBUS_IFACE, this);
    this._dbus.export(Gio.DBus.session, '/org/gnome/Shell/Extensions/ShuttlePro');
  }

  disable() {
    this._dbus.flush();
    this._dbus.unexport();
    delete this._dbus;
  }

  // Info returns the current shell state as a single JSON object:
  //
  //   {
  //     "title":    <string|null>,  focused window title
  //     "wm_class": <string|null>,  focused window WM_CLASS
  //     "x":        <int>,          current pointer x
  //     "y":        <int>          current pointer y
  //   }
  //
  // title and wm_class describe the focused window; if no window is focused
  // they are null rather than an error, so callers can simply check for null.
  // x and y are the current pointer position and are always present.
  Info() {
    const [x, y] = global.get_pointer();

    const win = global.get_window_actors()
      .find(w => w.meta_window.has_focus());

    if (!win) {
      return JSON.stringify({
        title: null,
        wm_class: null,
        x: x,
        y: y,
      });
    }

    const mw = win.meta_window;
    return JSON.stringify({
      title: mw.get_title(),
      wm_class: mw.get_wm_class(),
      x: x,
      y: y,
    });
  }
}
