### fanoutloop: batch independent planner change reads

The planner reuses the shared pending-change batch reader for the current candidate frontier. It preserves the lazy first-delivery walk and complete native fallback for rename/copy ancestry, with head and file-count binding. Incomplete or failed observations remain conservative holds; all prior trust, authoring, pagination and queue checks remain.
