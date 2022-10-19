CREATE TABLE proc_times (
    id int NOT NULL AUTO_INCREMENT,
    data_size INT NOT NULL,
    proc_type ENUM('ngrams', 'qs'),
    num_items INT NOT NULL,
    proc_time float,
    PRIMARY KEY (id)
)

CREATE TABLE usage (
    corpus_id varchar(127) NOT NULL,
	structattr_name varchar(127) NOT NULL,
	num_used int NOT NULL DEFAULT 1,
	UNIQUE (corpus_id, structattr_name)
)

-- individual data tables for live attributes and n-grams
-- are created/dropped by MASM dynamically
