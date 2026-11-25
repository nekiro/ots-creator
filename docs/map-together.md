# Map together

Several people can edit one map at the same time. One of them hosts: the map stays on
their computer and the others connect to it directly. No server of this project is
involved, nothing is uploaded anywhere.

- **Host:** open the map, click the people icon next to the map tabs, **Host a map** and
  **Start hosting** (a password is optional). Copy the invite code and send it to the
  others. The session ends when the host ends it or closes the map; the host saves the map
  as usual.
- **Join:** **Join a shared map…** on the start screen (or the people icon), paste the
  invite code or type an address (`203.0.113.5:47816`). Sessions on your network are
  listed by themselves, and sessions you joined before show whether they are open now
  and join with one click. A joined map opens in a new tab; leaving keeps it as your own
  copy.
- **Live:** every edit, undo and redo shows to everyone at once, with houses, towns,
  waypoints and zones. Undo only takes back your own edits and keeps the tiles others
  changed since. The others' cursors show on the map with their names and brushes, and the
  session panel lists who is there (click a name to go to them) with a chat.
- **Same data:** everyone needs the same client and the same items (`items.otb` or client
  ids); the editor refuses to join otherwise. Brushes may differ: edits are shared as the
  resulting tiles.
- **Reaching the host:** on the same network it just works. Over the internet the host's
  router must forward TCP port 47816 (the default): the editor asks the router to do it by
  UPnP and finds the host's internet address with a public STUN server, and both go into
  the invite code. When the router cannot be reached from the internet (carrier-grade NAT,
  UPnP off), all join one VPN such as Tailscale, ZeroTier or Radmin VPN: the invite code
  includes the VPN address too.
- **Security:** the connection is encrypted (TLS). The invite code carries the
  fingerprint of the host's certificate, so a guest only connects to the host that invited
  them. Your system may ask to let OTS Creator accept connections when you host: allow it.
  Sessions are found on the network on UDP port 47817.

Two people drawing different grounds right next to each other at the same moment may get
a wrong border between them; drawing it again fixes it.
