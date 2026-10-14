import assert from 'node:assert/strict';
import test from 'node:test';

import {
  activityMatchesProjectScope,
  applyUpdatedActivity,
  applyUpdatedTask,
  createCoalescedRefetch,
  taskMatchesBoardScope,
} from '../src/lib/taskUpdatedEvent.ts';

const baseScope = {
  selectedProjectId: 'all',
  bookmarkedProjectIds: ['proj-a'],
  searchQuery: '',
};

const sampleTask = (overrides = {}) => ({
  id: 'task-1',
  key: 'T-1',
  projectId: 'proj-a',
  projectIds: ['proj-a'],
  title: 'Original title',
  description: 'Some description',
  status: 'to_clarify',
  priority: 'medium',
  assignee: 'Alice',
  labels: ['frontend'],
  comments: [],
  createdAt: '2026-10-01T00:00:00Z',
  updatedAt: '2026-10-01T00:00:00Z',
  ...overrides,
});

const sampleActivity = (overrides = {}) => ({
  id: 'act-1',
  taskId: 'task-1',
  taskKey: 'T-1',
  taskTitle: 'Original title',
  projectId: 'proj-a',
  skillId: 'clarify',
  skillName: 'Clarify',
  action: 'clarify',
  status: 'running',
  summary: '',
  output: '',
  steps: [],
  createdAt: '2026-10-01T00:00:00Z',
  ...overrides,
});

test('applyUpdatedTask updates an existing task in place without extra fetches', () => {
  const existing = sampleTask();
  const updated = sampleTask({ title: 'Updated title', status: 'to_specify' });
  const next = applyUpdatedTask([existing], updated, baseScope);
  assert.equal(next.length, 1);
  assert.equal(next[0].title, 'Updated title');
  assert.equal(next[0].status, 'to_specify');
});

test('applyUpdatedTask removes an existing task when it moves out of the active board filter', () => {
  const existing = sampleTask({ assignee: 'Alice' });
  const updated = sampleTask({ assignee: 'Bob' });
  const next = applyUpdatedTask([existing], updated, {
    ...baseScope,
    assigneeFilter: 'Alice',
  });
  assert.equal(next.length, 0);
});

test('applyUpdatedTask prepends a newly matching task and skips out-of-scope tasks', () => {
  const updatedInScope = sampleTask({ id: 'task-2', key: 'T-2', projectId: 'proj-a', projectIds: ['proj-a'] });
  const updatedOutOfScope = sampleTask({ id: 'task-3', key: 'T-3', projectId: 'proj-b', projectIds: ['proj-b'] });

  const withInScope = applyUpdatedTask([sampleTask()], updatedInScope, baseScope);
  assert.equal(withInScope.length, 2);
  assert.equal(withInScope[0].id, 'task-2');

  const withOutOfScope = applyUpdatedTask([sampleTask()], updatedOutOfScope, baseScope);
  assert.equal(withOutOfScope.length, 1);
  assert.equal(withOutOfScope[0].id, 'task-1');
});

test('taskMatchesBoardScope checks bookmarks, project, sprint, label, and search query', () => {
  const t = sampleTask({
    sprint: 'Sprint 1',
    labels: ['bug', 'urgent'],
  });
  assert.equal(
    taskMatchesBoardScope(t, {
      ...baseScope,
      selectedProjectId: 'proj-a',
      sprintFilter: 'Sprint 1',
      labelFilter: 'urgent',
      searchQuery: 'original',
    }),
    true
  );
  assert.equal(
    taskMatchesBoardScope(t, {
      ...baseScope,
      sprintFilter: 'Sprint 2',
    }),
    false
  );
});

test('applyUpdatedActivity updates running/completed stats and prepends or updates activities in scope', () => {
  const initialStats = {
    total: 5,
    queued: 0,
    running: 1,
    completed: 4,
    failed: 0,
    canceled: 0,
  };
  const runningAct = sampleActivity({ status: 'running' });
  const completedAct = sampleActivity({ status: 'completed' });

  const res = applyUpdatedActivity([runningAct], initialStats, completedAct, 'all');
  assert.equal(res.activities.length, 1);
  assert.equal(res.activities[0].status, 'completed');
  assert.equal(res.stats.running, 0);
  assert.equal(res.stats.completed, 5);
});

test('activityMatchesProjectScope respects selected project', () => {
  assert.equal(
    activityMatchesProjectScope(sampleActivity({ projectId: 'proj-a' }), 'proj-a'),
    true
  );
  assert.equal(
    activityMatchesProjectScope(sampleActivity({ projectId: 'proj-b' }), 'proj-a'),
    false
  );
});

test('createCoalescedRefetch coalesces burst schedules into a single run after delayMs', async () => {
  let calls = 0;
  const coalesced = createCoalescedRefetch(10, () => {
    calls += 1;
  });

  coalesced.schedule();
  coalesced.schedule();
  coalesced.schedule();

  assert.equal(calls, 0);
  await new Promise(resolve => setTimeout(resolve, 25));
  assert.equal(calls, 1);
});
