#!/usr/bin/env python
import celery
import requests

celery_conf = dict(
	BROKER_URL = 'redis://192.168.1.34:6379/12',
    CELERY_RESULT_BACKEND = 'redis://192.168.1.34:6379/12',
	CELERY_TASK_SERIALIZER = 'json',
	CELERY_RESULT_SERIALIZER = 'json',
	CELERY_ACCEPT_CONTENT = ['json'])

MASM_API_RESTART_URL = 'http://kontext5:8080/kontext-services/soft-reset-all'

def clean_cache(corpname):
    worker = celery.Celery('bgcalc', config_source=celery_conf)
    # args=(ttl, subdir, dry_run, corpus_id)
    ans = worker.send_task('conc_cache.conc_cache_cleanup', args=(0, '', False, corpname))
    task_result = ans.get()
    # response: {'deleted': 4, 'type': 'summary', 'processed': 4}
    return task_result

def reload_service():
    ans = requests.post(MASM_API_RESTART_URL)
    if ans.status_code == 200:
        return ans.json()
    else:
        ans.raise_for_status()

if __name__ == '__main__':
    import sys
    print(clean_cache(sys.argv[1]))
    print(reload_service())
