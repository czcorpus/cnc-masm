# cnc-masm

A set of REST services for CNC corpus data manipulation and KonText monitoring

Functions:

1. corpus data information (indices location and modification datetime, basic registry configuration, liveattrs database info)
1. KonText process monitoring - process availability and restart notification
1. creating live-attributes for KonText
1. corpus data synchronization between `/cnk/run/manatee/data` and `/cnk/local/ssd/run/manatee/data`
   (or any other configured location)