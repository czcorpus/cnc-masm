# cnc-masm

A set of REST services for CNC Manatee data and assets (including "live-attrs")

Functions:

1. corpus data information (indices location and modification datetime, basic registry configuration, liveattrs database info)
2. creating and searching in live-attributes (used by KonText)
3. corpus data synchronization between `/cnk/run/manatee/data` and `/cnk/local/ssd/run/manatee/data`
   (or any other configured location)

## API

see [API.md](./API.md)