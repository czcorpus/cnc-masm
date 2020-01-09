#!/usr/bin/env python3
import celery
import requests

"""
This script can be used to refresh information about a corpus
in case the corpus data changed (typically a 'monitor' corpus).

Please do not forget to check/update the configuration constants
below to match your actual setup.

Usage:

a) as an imported module:

import kontext_reset
refresh_corpus_props('corpus_name')

b) via command line:

./kontext_reset.py corpus_name
"""

celery_conf = dict(
	BROKER_URL = 'redis://192.168.1.34:6379/12',
    CELERY_RESULT_BACKEND = 'redis://192.168.1.34:6379/12',
	CELERY_TASK_SERIALIZER = 'json',
	CELERY_RESULT_SERIALIZER = 'json',
	CELERY_ACCEPT_CONTENT = ['json'])

MASM_API_RESTART_URL = 'http://kontext5:8080/kontext-services/soft-reset-all'
MASM_API_CORPUS_DB_UPDATE_URL = 'http://kontext5:8080/corpora-database/{}/auto-update'

def _clean_cache(corpname):
    worker = celery.Celery('bgcalc', config_source=celery_conf)
    # args=(ttl, subdir, dry_run, corpus_id)
    ans = worker.send_task('conc_cache.conc_cache_cleanup', args=(0, '', False, corpname))
    task_result = ans.get()
    # response: {'deleted': 4, 'type': 'summary', 'processed': 4}
    return task_result

def _reload_service():
    ans = requests.post(MASM_API_RESTART_URL)
    if ans.status_code == 200:
        return ans.json()
    else:
        ans.raise_for_status()

def _update_corpora_db(corpname):
    ans = requests.post(MASM_API_CORPUS_DB_UPDATE_URL.format(corpname))
    if ans.status_code == 200:
        return ans.json()
    else:
        ans.raise_for_status()

def refresh_corpus_props(corpname):
    """
    The function performs all the necessary
    steps to refresh information about corpus.
    """
    return dict(
        clean_cache=_clean_cache(corpname),
        reload_service=_reload_service(),
        update_corpora_db=_update_corpora_db(corpname)
    )


if __name__ == '__main__':
    import sys
    if len(sys.argv) < 2:
        print('Missing corpus name')
        sys.exit(1)
    try:
        print(refresh_corpus_props(sys.argv[1]))
    except Exception as e:
        print('{}: {}'.format(e.__class__.__name__, e))
        sys.exit(1)
