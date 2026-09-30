// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Status, StatusLabel, TriggerType } from '@/api/v1/schema';
import dayjs from '../../lib/dayjs';
import { AppBarContext } from '@/contexts/AppBarContext';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import { SearchStateProvider } from '@/contexts/SearchStateContext';
import { WorkspaceKind } from '@/lib/workspace';
import { usePaginatedDAGRuns } from '../../features/dag-runs/hooks/dagRunPagination';
import { useClient } from '../../hooks/api';
import DashboardTimeChart from '../../features/dashboard/components/DashboardTimechart';
import DashboardPage from '../index';

vi.mock('../../features/dashboard/components/DashboardTimechart', () => ({
  default: vi.fn(() => <div data-testid="dashboard-timechart" />),
}));

vi.mock('../../features/dag-runs/components/dag-run-details', () => ({
  DAGRunDetailsModal: () => null,
}));

vi.mock('../../features/dags/components/common', () => ({
  CreateDAGModal: () => <button type="button">New workflow</button>,
}));

vi.mock('../../features/dag-runs/hooks/dagRunPagination', () => ({
  usePaginatedDAGRuns: vi.fn(),
}));

vi.mock('../../hooks/api', () => ({
  useClient: vi.fn(),
}));

const useClientMock = vi.mocked(useClient);
const usePaginatedDAGRunsMock = vi.mocked(usePaginatedDAGRuns);
const dashboardTimeChartMock = vi.mocked(DashboardTimeChart);

const historicalRun: ReturnType<typeof usePaginatedDAGRuns>['dagRuns'][number] =
  {
    name: 'etl',
    dagRunId: 'old-run',
    status: Status.Success,
    statusLabel: StatusLabel.succeeded,
    artifactsAvailable: false,
    autoRetryCount: 0,
    triggerType: TriggerType.manual,
    queuedAt: '2026-04-02T00:00:00Z',
    scheduleTime: '',
    startedAt: '2026-04-02T00:00:00Z',
    finishedAt: '2026-04-02T00:01:00Z',
  };

function mockDAGRuns(
  dagRuns: ReturnType<typeof usePaginatedDAGRuns>['dagRuns'] = []
) {
  usePaginatedDAGRunsMock.mockReturnValue({
    dagRuns,
    headPage: undefined,
    error: null,
    isInitialLoading: false,
    isLoadingMore: false,
    loadMoreError: null,
    hasMore: false,
    refresh: vi.fn(),
    loadMore: vi.fn(),
  });
}

function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    apiURL: '/api/v1',
    basePath: '/',
    title: 'Dagu',
    navbarColor: '',
    tz: 'UTC',
    tzOffsetInSec: 0,
    version: 'test',
    maxDashboardPageLimit: 100,
    remoteNodes: 'local,remote-a',
    initialWorkspaces: [],
    authMode: 'none',
    setupRequired: false,
    oidcEnabled: false,
    oidcButtonLabel: '',
    proxyEnabled: false,
    proxyButtonLabel: '',
    terminalEnabled: false,
    gitSyncEnabled: false,
    updateAvailable: false,
    latestVersion: '',
    permissions: {
      writeDags: true,
      runDags: true,
    },
    license: {
      valid: true,
      plan: 'community',
      expiry: '',
      features: [],
      gracePeriod: false,
      community: true,
      source: 'test',
      warningCode: '',
    },
    paths: {
      dagsDir: '',
      logDir: '',
      suspendFlagsDir: '',
      adminLogsDir: '',
      baseConfig: '',
      dagRunsDir: '',
      queueDir: '',
      procDir: '',
      serviceRegistryDir: '',
      configFileUsed: '',
      gitSyncDir: '',
      auditLogsDir: '',
    },
    ...overrides,
  };
}

function renderPage({
  selectedWorkspace = '',
}: { selectedWorkspace?: string } = {}) {
  return render(
    <MemoryRouter initialEntries={['/dashboard']}>
      <ConfigContext.Provider value={makeConfig()}>
        <SearchStateProvider>
          <AppBarContext.Provider
            value={{
              title: '',
              setTitle: () => undefined,
              remoteNodes: ['local', 'remote-a'],
              setRemoteNodes: () => undefined,
              selectedRemoteNode: 'remote-a',
              selectRemoteNode: () => undefined,
              workspaces: selectedWorkspace
                ? [{ id: 'workspace-1', name: selectedWorkspace }]
                : [],
              workspaceSelection: selectedWorkspace
                ? {
                    kind: WorkspaceKind.workspace,
                    workspace: selectedWorkspace,
                  }
                : { kind: WorkspaceKind.all },
              selectWorkspace: () => undefined,
            }}
          >
            <DashboardPage />
          </AppBarContext.Provider>
        </SearchStateProvider>
      </ConfigContext.Provider>
    </MemoryRouter>
  );
}

describe('DashboardPage', () => {
  const clientGetMock = vi.fn();

  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    clientGetMock.mockReset();
    useClientMock.mockReturnValue({
      GET: clientGetMock.mockResolvedValue({
        data: {
          dags: [],
          pagination: {
            totalPages: 1,
            totalRecords: 0,
          },
        },
      }),
    } as never);
    mockDAGRuns();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('requests dashboard DAG runs without queued status', async () => {
    renderPage();

    await waitFor(() => {
      expect(usePaginatedDAGRunsMock).toHaveBeenCalled();
    });

    const latestCall =
      usePaginatedDAGRunsMock.mock.calls[
        usePaginatedDAGRunsMock.mock.calls.length - 1
      ]?.[0];
    expect(latestCall).toBeDefined();
    if (!latestCall) {
      throw new Error('Expected dashboard to request paginated DAG runs');
    }

    const latestQuery = latestCall.query;
    expect(latestQuery).toBeDefined();
    if (!latestQuery) {
      throw new Error('Expected dashboard DAG run query to be defined');
    }

    expect(latestQuery).toEqual(
      expect.objectContaining({
        remoteNode: 'remote-a',
        status: [
          Status.Success,
          Status.Failed,
          Status.Running,
          Status.Aborted,
          Status.NotStarted,
          Status.PartialSuccess,
          Status.Waiting,
          Status.Rejected,
        ],
      })
    );
    expect(latestQuery.status).not.toContain(Status.Queued);
  });

  it('scopes dashboard DAG and DAG-run requests by selected workspace', async () => {
    renderPage({ selectedWorkspace: 'ops' });

    await waitFor(() => {
      expect(usePaginatedDAGRunsMock).toHaveBeenCalled();
      expect(clientGetMock).toHaveBeenCalled();
    });

    const latestCall =
      usePaginatedDAGRunsMock.mock.calls[
        usePaginatedDAGRunsMock.mock.calls.length - 1
      ]?.[0];
    expect(latestCall?.query).toEqual(
      expect.objectContaining({
        remoteNode: 'remote-a',
        workspace: 'ops',
      })
    );

    expect(clientGetMock).toHaveBeenCalledWith(
      '/dags',
      expect.objectContaining({
        params: {
          query: expect.objectContaining({
            remoteNode: 'remote-a',
            workspace: 'ops',
          }),
        },
      })
    );
  });

  function mockDAGInventory(names: string[], totalRecords: number) {
    clientGetMock.mockResolvedValue({
      data: {
        dags: names.map((name) => ({ fileName: `${name}.yaml`, dag: { name } })),
        pagination: {
          totalPages: 1,
          totalRecords,
        },
      },
    });
  }

  it('invites creating the first workflow when no workflows exist', async () => {
    renderPage();

    expect(
      await screen.findByText('Create your first workflow')
    ).toBeVisible();
    expect(screen.getByText('New workflow')).toBeVisible();
    expect(screen.queryByTestId('dashboard-timechart')).not.toBeInTheDocument();
  });

  it('points at the Workflows page when workflows exist but nothing ran', async () => {
    mockDAGInventory(['etl', 'backup'], 2);

    renderPage();

    expect(await screen.findByText(/No runs on /)).toBeVisible();
    expect(screen.getByText(/Start a workflow from the/)).toBeVisible();
    expect(screen.queryByTestId('dashboard-timechart')).not.toBeInTheDocument();
    expect(
      screen.queryByText('Create your first workflow')
    ).not.toBeInTheDocument();
  });

  it('suggests the example workflows when only seeded examples exist', async () => {
    mockDAGInventory(['example-01-basic-sequential'], 1);

    renderPage();

    expect(
      await screen.findByText(/Run one of the example workflows/)
    ).toBeVisible();
  });

  it('keeps the timechart when runs exist', async () => {
    mockDAGInventory(['etl'], 1);
    mockDAGRuns([historicalRun]);

    renderPage();

    expect(await screen.findByTestId('dashboard-timechart')).toBeVisible();
    expect(screen.queryByText(/No runs on /)).not.toBeInTheDocument();
  });

  it('offers a retry instead of first-run guidance when the inventory request fails', async () => {
    clientGetMock.mockResolvedValue({});

    renderPage();

    expect(
      await screen.findByText('Failed to load the workflow list.')
    ).toBeVisible();
    expect(
      screen.queryByText('Create your first workflow')
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/No runs on /)).not.toBeInTheDocument();

    const callsBeforeRetry = clientGetMock.mock.calls.length;
    mockDAGInventory(['etl'], 1);
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

    await waitFor(() => {
      expect(clientGetMock.mock.calls.length).toBeGreaterThan(callsBeforeRetry);
    });
    expect(await screen.findByText(/No runs on /)).toBeVisible();
  });

  function latestDashboardQuery() {
    const calls = usePaginatedDAGRunsMock.mock.calls;
    const latest = calls[calls.length - 1]?.[0];
    if (!latest?.query) {
      throw new Error('Expected dashboard DAG run query to be defined');
    }
    return latest.query;
  }

  it('scopes the DAG-run query to the selected date-range preset', async () => {
    const user = userEvent.setup();
    renderPage();

    await waitFor(() => {
      expect(usePaginatedDAGRunsMock).toHaveBeenCalled();
    });

    // The default preset is today: start-of-day lower bound, open upper bound.
    expect(latestDashboardQuery().fromDate).toBe(
      dayjs().utcOffset(0).startOf('day').unix()
    );
    expect(latestDashboardQuery().toDate).toBeUndefined();

    await user.click(screen.getByRole('combobox', { name: 'Date range' }));
    await user.click(
      await screen.findByRole('option', { name: 'Last 7 days' })
    );

    await waitFor(() => {
      expect(latestDashboardQuery().fromDate).toBe(
        dayjs().utcOffset(0).startOf('day').subtract(7, 'day').unix()
      );
    });
    expect(latestDashboardQuery().toDate).toBeUndefined();
  });

  it('queries all history without framing the chart at the epoch', async () => {
    const user = userEvent.setup();
    mockDAGInventory(['etl'], 1);
    mockDAGRuns([historicalRun]);
    renderPage();

    await waitFor(() => {
      expect(usePaginatedDAGRunsMock).toHaveBeenCalled();
    });

    await user.click(screen.getByRole('combobox', { name: 'Date range' }));
    await user.click(await screen.findByRole('option', { name: 'All time' }));

    await waitFor(() => {
      expect(latestDashboardQuery().fromDate).toBe(0);
    });
    expect(latestDashboardQuery().toDate).toBeUndefined();
    expect(dashboardTimeChartMock.mock.lastCall?.[0].selectedDate).toEqual({
      startTimestamp: dayjs(historicalRun.startedAt).unix(),
    });
  });

  it('queries all history after clearing both custom bounds', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole('combobox', { name: 'Date range' }));
    await user.click(await screen.findByRole('option', { name: 'Custom' }));
    const inputs = await screen.findAllByPlaceholderText('YYYY-MM-DD HH:mm:ss');
    fireEvent.change(inputs[1]!, {
      target: { value: '2026-05-01 00:00:00' },
    });
    fireEvent.change(inputs[0]!, { target: { value: '' } });
    expect(latestDashboardQuery().fromDate).toBeUndefined();
    expect(latestDashboardQuery().toDate).toBe(
      dayjs('2026-05-01T00:00').utcOffset(0, true).unix()
    );

    fireEvent.change(inputs[1]!, { target: { value: '' } });
    expect(latestDashboardQuery().fromDate).toBe(0);
    expect(latestDashboardQuery().toDate).toBeUndefined();
    expect((inputs[0] as HTMLInputElement).value).toBe('');
    expect((inputs[1] as HTMLInputElement).value).toBe('');
  });

  it('restores all-time history without storing an epoch bound', async () => {
    const scope = `dashboard:${JSON.stringify({
      remoteNode: 'remote-a',
      workspace: 'workspace:all',
    })}`;
    sessionStorage.setItem(
      'dagu.searchState',
      JSON.stringify({
        [scope]: { selectedDAGRun: 'all', datePreset: 'all', dateRange: {} },
      })
    );
    renderPage();

    await waitFor(() => {
      expect(latestDashboardQuery().fromDate).toBe(0);
    });
    expect(latestDashboardQuery().toDate).toBeUndefined();
    expect(
      JSON.parse(sessionStorage.getItem('dagu.searchState')!)[scope]
    ).toEqual({
      selectedDAGRun: 'all',
      datePreset: 'all',
      dateRange: {},
    });
  });

  it('applies a custom range through the date-range picker', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole('combobox', { name: 'Date range' }));
    await user.click(await screen.findByRole('option', { name: 'Custom' }));

    const boundInputs = await screen.findAllByPlaceholderText(
      'YYYY-MM-DD HH:mm:ss'
    );
    expect(boundInputs).toHaveLength(2);

    fireEvent.change(boundInputs[0] as HTMLElement, {
      target: { value: '2026-05-01 08:30:00' },
    });

    await waitFor(() => {
      expect(latestDashboardQuery().fromDate).toBe(
        dayjs('2026-05-01T08:30').utcOffset(0, true).unix()
      );
    });
  });

  // A preset means "relative to now", so a session left open across a date
  // boundary must not keep querying the range it computed back then.
  it('resolves a stored date preset relative to now', async () => {
    window.sessionStorage.setItem(
      'dagu.searchState',
      JSON.stringify({
        [`dashboard:${JSON.stringify({
          remoteNode: 'remote-a',
          workspace: 'workspace:all',
        })}`]: {
          selectedDAGRun: 'all',
          datePreset: 'last30days',
          dateRange: { startDate: 1, endDate: 2 },
        },
      })
    );

    renderPage();

    await waitFor(() => {
      expect(latestDashboardQuery().fromDate).toBe(
        dayjs().utcOffset(0).startOf('day').subtract(30, 'day').unix()
      );
    });
    expect(latestDashboardQuery().toDate).toBeUndefined();
  });

  // Sessions stored before the preset selector existed carry only a concrete
  // range; it is kept and shown as a custom range.
  it('keeps a stored concrete range as a custom range', async () => {
    const startDate = dayjs('2026-04-02T00:00').utcOffset(0, true).unix();
    window.sessionStorage.setItem(
      'dagu.searchState',
      JSON.stringify({
        [`dashboard:${JSON.stringify({
          remoteNode: 'remote-a',
          workspace: 'workspace:all',
        })}`]: {
          selectedDAGRun: 'all',
          dateRange: { startDate, endDate: 1780000000 },
        },
      })
    );

    renderPage();

    await waitFor(() => {
      expect(latestDashboardQuery().fromDate).toBe(startDate);
    });
    expect(latestDashboardQuery().toDate).toBe(1780000000);
    expect(
      await screen.findAllByPlaceholderText('YYYY-MM-DD HH:mm:ss')
    ).toHaveLength(2);
  });

  it('shows placeholders instead of zeros while runs load', async () => {
    mockDAGInventory(['etl'], 1);
    usePaginatedDAGRunsMock.mockReturnValue({
      dagRuns: [],
      headPage: undefined,
      error: null,
      isInitialLoading: true,
      isLoadingMore: false,
      loadMoreError: null,
      hasMore: false,
      refresh: vi.fn(),
      loadMore: vi.fn(),
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(4);
    });
    expect(screen.queryByText(/No runs on /)).not.toBeInTheDocument();
  });
});
