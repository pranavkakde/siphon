db = db.getSiblingDB('siphon');
db.siphon_results.createIndex( { "execution_id": 1, "test_case_id": 1 }, { unique: true } );
