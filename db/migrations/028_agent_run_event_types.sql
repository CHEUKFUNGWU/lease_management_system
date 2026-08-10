-- Migration 028: allow Gateway and Pi-like Runner lifecycle/control events.

ALTER TABLE ai_chat_run_events
    DROP CONSTRAINT IF EXISTS ai_chat_run_events_event_type_check;

ALTER TABLE ai_chat_run_events
    ADD CONSTRAINT ai_chat_run_events_event_type_check CHECK (event_type IN (
        'message_start', 'message_delta', 'message_end',
        'tool_start', 'tool_update', 'tool_end', 'tool_execution',
        'tool_started', 'tool_completed',
        'review_prompt', 'artifact_ready', 'queue_update',
        'run_status', 'run_started', 'run_resumed', 'run_steered',
        'run_steer', 'run_follow_up', 'run_branch_created',
        'checkpoint_restored', 'run_end', 'run_finished',
        'run_error', 'run_failed', 'run_cancelled'
    ));
