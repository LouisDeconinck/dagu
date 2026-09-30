// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import { DateRangePicker } from '../date-range-picker';

function renderPicker(
  onFromDateChange = vi.fn(),
  fromDate = '2026-09-01T00:00'
) {
  render(
    <DateRangePicker
      fromDate={fromDate}
      toDate="2026-09-30T23:59"
      onFromDateChange={onFromDateChange}
      onToDateChange={vi.fn()}
      fromLabel="From"
      toLabel="To"
    />
  );
  return screen.getAllByPlaceholderText('YYYY-MM-DD HH:mm:ss')[0]!;
}

describe('DateRangePicker', () => {
  it('reports an emptied field as a cleared bound', () => {
    const onFromDateChange = vi.fn();
    const input = renderPicker(onFromDateChange);

    fireEvent.change(input, { target: { value: '' } });

    expect(onFromDateChange).toHaveBeenCalledWith('');
  });

  // A half-typed date is not a bound, so it must not be reported as one, and
  // must not be mistaken for a clear either.
  it('reports nothing while a date is partially typed', () => {
    const onFromDateChange = vi.fn();
    const input = renderPicker(onFromDateChange);

    fireEvent.change(input, { target: { value: '2026-09-1' } });

    expect(onFromDateChange).not.toHaveBeenCalled();
  });

  it('reports a whole date once it parses', () => {
    const onFromDateChange = vi.fn();
    const input = renderPicker(onFromDateChange);

    fireEvent.change(input, { target: { value: '2026-08-15 09:30:00' } });

    expect(onFromDateChange).toHaveBeenCalledWith('2026-08-15T09:30');
  });

  it('preserves a wall-clock value during the browser DST gap', () => {
    const input = renderPicker(vi.fn(), '2026-03-08T02:30');

    expect(input).toHaveValue('2026-03-08 02:30:00');
  });

  it('adjusts wall-clock hours across the browser DST gap', () => {
    const onFromDateChange = vi.fn();
    const input = renderPicker(
      onFromDateChange,
      '2026-03-08T01:30'
    ) as HTMLInputElement;
    input.setSelectionRange(12, 12);

    fireEvent.keyDown(input, { key: 'ArrowUp' });

    expect(onFromDateChange).toHaveBeenCalledWith('2026-03-08T02:30:00');
  });

  it('keeps calendar rollover when adjusting a month', () => {
    const onFromDateChange = vi.fn();
    const input = renderPicker(
      onFromDateChange,
      '2026-01-31T12:00'
    ) as HTMLInputElement;
    input.setSelectionRange(6, 6);

    fireEvent.keyDown(input, { key: 'ArrowUp' });

    expect(onFromDateChange).toHaveBeenCalledWith('2026-03-03T12:00:00');
  });

  it.each([
    '2026-02-30 12:00:00',
    '2026-13-01 12:00:00',
    '2026-03-01 24:00:00',
    '2026-03-01 12:60:00',
    '2026-03-01 12:00:60',
  ])('keeps invalid input %s uncommitted', (value) => {
    const onFromDateChange = vi.fn();
    const input = renderPicker(onFromDateChange);

    fireEvent.change(input, { target: { value } });

    expect(input).toHaveValue(value);
    expect(onFromDateChange).not.toHaveBeenCalled();
  });

  it('accepts a leap-day input', () => {
    const onFromDateChange = vi.fn();
    const input = renderPicker(onFromDateChange);

    fireEvent.change(input, { target: { value: '2024-02-29 12:00:00' } });

    expect(onFromDateChange).toHaveBeenCalledWith('2024-02-29T12:00');
  });
});
