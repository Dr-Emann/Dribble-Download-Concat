# dribble_download_concat

This is a very niche program.
The original motivation for this program was for downloading [Google Takeout] archives, and piping to tar.
The URLs for takeout timeout quite quickly, so downloading sequentially doesn't work.
Instead, this slowly (4KiB every 5 seconds) downloads from all passed urls,
only downloading the first non-complete download at full speed.

[Google Takeout]: https://takeout.google.com