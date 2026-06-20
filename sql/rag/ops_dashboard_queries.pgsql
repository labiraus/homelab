\echo 'Document lifecycle throughput (last 1 hour)'
SELECT
    subject,
    COUNT(*) AS event_count
FROM rag.document_lifecycle_events
WHERE occurred_at >= NOW() - INTERVAL '1 hour'
GROUP BY subject
ORDER BY subject;

\echo ''
\echo 'Recent processor failures'
SELECT
    document_id,
    processing_version,
    occurred_at,
    event_payload ->> 'error' AS error
FROM rag.document_lifecycle_events
WHERE subject = 'documents.events.processor.failed'
ORDER BY occurred_at DESC
LIMIT 20;

\echo ''
\echo 'Current document status counts'
SELECT
    status,
    COUNT(*) AS document_count
FROM rag.documents
GROUP BY status
ORDER BY status;

\echo ''
\echo 'Assistant proposal outcomes (last 7 days)'
SELECT
    status,
    COUNT(*) AS proposal_count
FROM assistant.file_proposals
WHERE created_at >= NOW() - INTERVAL '7 days'
GROUP BY status
ORDER BY status;
