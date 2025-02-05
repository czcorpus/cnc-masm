# CNC-MASM API

Notes: all the functions return JSON and in case there are HTTP body arguments,
we mean a JSON object with respective attributes.

## corpora

:orange_circle:  `GET /corpora/[corpus ID]`
(`GET /corpora/[sub dir.]/[corpus ID]`)

Get information about corpus files.

TODO


:orange_circle: `POST /corpora/[corpus ID]/_syncData`
`POST /corpora/[sub dir.]/[corpus ID]/_syncData`

Synchronize data (`/cnc/run/manatee/data` vs `/cnk/local/ssd/run/manatee/data`)
for a corpus. Please note that this applies only for corpora configured in
`corporaSetup.syncAllowedCorpora`. In our case, this mostly applies for the
`online*` corpora. The method is able to determine which location (ssd vs distributed fs) has newer data and configure a respective `rsync` call accordingly.

## registry

TODO
