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

## liveAttributes

:orange_circle: `POST /liveAttributes/[corpus ID]/data`

Extract "live attributes" data based on provided arguments and corpus registry files. The function is asynchronous and returns an information about a respective job.

URL Arguments:

* `noCache` - if `1` then MASM will generate a new version of data extraction configuration. Otherwise, the currently stored config will be used. In case there no configuration yet, a new one will be created automatically even if `noCache` is not specified.
* `atomStructure` specifies the "minimal" structure we want to register. This is needed only if `SUBCORPATTRS` mention more than one structure.
* `bibIdAttr` (optional) - specifies a structural attribute uniquely identifying each live attributes entry (typically, something like `doc.id`). In case this is defined, MASM can provide a "bibliographical" entry overview (e.g. individual book, article etc.)
* `mergeAttr` (optional) a structural attribute specifying a "join" attribute used for registering aligned structures (typically - sentences).
* `mergeFn` (required if `mergeAttr` is used) - in some cases, there is no attribute value across multiple aligned items which can be used without modification, it is obligatory to specify a transformation function for such values. This is mostly an issue in case of InterCorp where we have a good "join" candidate but the values looks like this: `cs:foo` vs. `en:foo`. Specifying `mergeFn=intercorp` will automatically strip the language code prefix and leave us with a usable "join" attribute. There is also `mergeFn=identity` for case where the attribute can be used without a change.
* `append` (optional) - normally, calling `POST data` will drop a respective database table. To be able to generate data for InterCorp and other aligned corpora where all the corpora are in a single table, `append=1` must be specified for 2nd and further processed corpora.
* `noCorpusUpdate` (optional) - by default, generating new live attributes also performs two addtional actions to make sure KonText knows about new/updated liveattrs. The actions are: 1. update of text_types_db column in the `corpora` table of CNC's database, 2. triggering cache reset on the KonText side. To disable this step, just set `noCorpusUpdate=1`.
* `skipNgrams` - if `1` then n-grams won't be generated even if they are (pre)configured
(either via previous `PUT /liveAttributes/{corpusId}/conf` or by passing JSON args with n-gram
configuration). In case the setting cannot have an effect (= n-grams are not configured),
the setting is silently ignored.

BODY arguments (JSON):

* `verticalFiles Array<string>` - ad-hoc paths to vertical files to be processed. This supresses any other vertical file specification (registry, masm vertical file search). But the value is not written to a respective data extraction config.
This is used e.g. for CNC's `online*` corpora where in some cases, each day a set of (sub)vertical files is different.
* `ngrams`
  * `ngrams.vertColumns` - defines a list of columns to extract from vertical file
    * `ngrams.vertColumns[i].idx` - column index (starting from zero)
    * `ngrams.vertColumns[i].transformFn` - a function name (from a predefined list of items) to transform value (e.g. `toLower`, `firstChar`)
  * `ngrams.ngramSize` (1 = unigram, 2 = bigram, ...)
  * `ngrams.calcARF` - boolean value; please note that calculating ARF requires two-pass processing of a respective vertical file

:orange_circle: `DELETE /liveAttributes/[corpus ID]/data`

This call deletes all the data and table for the corpus.


:orange_circle: `GET /liveAttributes/[corpus ID]/conf`

Returns an existing live attributes extraction config. In case nothing is defined (yet) it returns code 404.

:orange_circle: `PUT /liveAttributes/[corpus ID]/conf`

Create a new configuration (just like in case of `POST data` but without data processing).

URL arguments:

* `atomStructure` (see `POST data`)
* `bibIdAttr` (see `POST data`)
* `mergeAttr` (see `POST data`)
* `mergeFn` (see `POST data`)

:orange_circle: `POST /liveAttributes/[corpus ID]/query`

Search available values of a group of attributes based on provided values of a
different group of attributes.

BODY arguments (JSON):

* `aligned Array<string>`
* `attrs {[attr:string]:Array<string>}`
* `autocompleteAttr string`
* `maxAttrListSize number`


:orange_circle: `POST /liveAttributes/[corpus ID]/fillAttrs`

For a structural attribute and its values, find values of different structural attributes specified in fill list (see BODY args).

BODY arguments (JSON):

* `search:string`
* `values:Array<string>`
* `fill:Array<string>`

:orange_circle: `POST /liveAttributes/[corpus ID]/selectionSubcSize`

Return a size (in tokens) of a subcorpus defined by selected attributes.

BODY arguments (JSON):

- see `POST query`

:orange_circle: `POST /liveAttributes/[corpus ID]/attrValAutocomplete`

BODY arguments (JSON):

The function is similar to the `POST query` but for one of provided attributes, it allows specifying an incomplete value. The function then returns all the matching values for the attribute (and also all the valid values for other attributes - just like `POST query`)

- see `POST query`

:orange_circle: `POST /liveAttributes/[corpus ID]/getBibliography`

BODY arguments (JSON):

* `itemId:string` - an unique identifier of the item (see bibIdAttr for more info)

:orange_circle: `POST /liveAttributes/[corpus ID]/findBibTitles`

BODY arguments (JSON):

* `itemIds:Array<string>`

:orange_circle: `GET /liveAttributes/[corpus ID]/stats`

For a corpus, return a map of structural attributes and numbers of queries for each one.

:orange_circle: `POST /liveAttributes/[corpus ID]/updateIndexes`

URL arguments:

* `maxColumns` - max. number of columns considered for creating indexes

:orange_circle: `POST /liveAttributes/[corpus ID]/mixSubcorpus`

Create a subcorpus matching provided text types and required ratios (0..1). Due to combinatorial
nature of the problem, the ratios may be slightly off and for more complex text type specification
the ratio error will probably increase.

BODY arguments (JSON):

    * `corpora:Array<string>` (main corpus plus possible aligned corpora)
    * `textTypes:Array<{attrName:string; attrValue:string; ratio:number}>`

Returned value (JSON):

```
{
    error:string;
    docIds:Array<string>;
    sizeAssembled:int;
    categorySizes:Array<int>;
}
```

:orange_circle: `POST /liveAttributes/[corpus ID]/ngrams`

Generate intermediate n-gram database for query suggestion engine.

Please note that to be able to run this function, core live attributes data must be extracted
(see `POST /liveAttributes/[corpus ID]/data`) with n-gram configuration (either with already pre-generated
configuration or with adhoc configuration in JSON body).

URL arguments:

    * `posColIdx` an index of vertical file column where Part of Speech can be obtained (it can
  be either a direct `pos` attribute or e.g. a tag from which the PoS can be extracted - e.g. `tag`)
    * `posTagset` a tagset identifier (e.g. `cs_cnc2020`, `cs_cnc2000_spk`)


:orange_circle: `POST /liveAttributes/[corpus ID]/querySuggestions`

From n-gram intermediate data (see `POST /liveAttributes/[corpus ID]/ngrams`), export query suggestion
data to a CouchDB database (masm will create the database, views and sets access to a globally
server-defined user).


## jobs

:orange_circle: `GET /jobs`

Return list of recent jobs (even finished ones).

URL arguments:

* `compact` - if `1` then the individual items are a bit pruned for better readability
* `unfinishedOnly` - if `1` then only running jobs will be listed

:orange_circle: `GET /jobs/[job ID]`

Return an information about a provided job.

:orange_circle: `DELETE /jobs/[job ID]`

Delete a job. In case it is running, MASM will kill the actual processing.


## registry

TODO
