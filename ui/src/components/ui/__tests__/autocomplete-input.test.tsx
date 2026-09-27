// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import { AutocompleteInput } from '../autocomplete-input';

function TestAutocomplete({
  suggestions = ['alpha', 'beta', 'alpine'],
  onValueChange = vi.fn(),
  onEnterPress = vi.fn(),
  initialValue = '',
}: {
  suggestions?: string[];
  onValueChange?: (value: string) => void;
  onEnterPress?: () => void;
  initialValue?: string;
}) {
  const [value, setValue] = React.useState(initialValue);
  return (
    <AutocompleteInput
      placeholder="Filter..."
      value={value}
      onValueChange={(v) => {
        setValue(v);
        onValueChange(v);
      }}
      onEnterPress={onEnterPress}
      suggestions={suggestions}
    />
  );
}

describe('AutocompleteInput', () => {
  it('shows all suggestions on focus', () => {
    render(<TestAutocomplete />);

    const input = screen.getByRole('combobox', { name: 'Filter...' });
    fireEvent.focus(input);

    expect(screen.getByRole('listbox')).toBeInTheDocument();
    expect(screen.getAllByRole('option')).toHaveLength(3);
  });

  it('filters suggestions by substring as the user types', () => {
    render(<TestAutocomplete />);

    const input = screen.getByRole('combobox', { name: 'Filter...' });
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: 'alp' } });

    const options = screen.getAllByRole('option');
    expect(options.map((o) => o.textContent)).toEqual(['alpha', 'alpine']);
  });

  it('hides the dropdown when no suggestion matches', () => {
    render(<TestAutocomplete />);

    const input = screen.getByRole('combobox', { name: 'Filter...' });
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: 'zzz' } });

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('selects a suggestion on click', () => {
    const onValueChange = vi.fn();
    render(<TestAutocomplete onValueChange={onValueChange} />);

    const input = screen.getByRole('combobox', { name: 'Filter...' });
    fireEvent.focus(input);
    fireEvent.click(screen.getByRole('option', { name: 'beta' }));

    expect(onValueChange).toHaveBeenLastCalledWith('beta');
    expect(input).toHaveValue('beta');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('selects the highlighted suggestion on Enter', () => {
    const onValueChange = vi.fn();
    const onEnterPress = vi.fn();
    render(
      <TestAutocomplete
        onValueChange={onValueChange}
        onEnterPress={onEnterPress}
      />
    );

    const input = screen.getByRole('combobox', { name: 'Filter...' });
    fireEvent.focus(input);
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    fireEvent.keyDown(input, { key: 'Enter' });

    expect(onValueChange).toHaveBeenLastCalledWith('alpha');
    expect(onEnterPress).not.toHaveBeenCalled();
  });

  it('submits the typed value on Enter when nothing is highlighted', () => {
    const onEnterPress = vi.fn();
    render(<TestAutocomplete onEnterPress={onEnterPress} />);

    const input = screen.getByRole('combobox', { name: 'Filter...' });
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: 'partial' } });
    fireEvent.keyDown(input, { key: 'Enter' });

    expect(onEnterPress).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('closes the dropdown on Escape', () => {
    render(<TestAutocomplete />);

    const input = screen.getByRole('combobox', { name: 'Filter...' });
    fireEvent.focus(input);
    expect(screen.getByRole('listbox')).toBeInTheDocument();
    fireEvent.keyDown(input, { key: 'Escape' });

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('closes the dropdown on outside click', () => {
    render(
      <div>
        <TestAutocomplete />
        <button type="button">outside</button>
      </div>
    );

    const input = screen.getByRole('combobox', { name: 'Filter...' });
    fireEvent.focus(input);
    expect(screen.getByRole('listbox')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole('button', { name: 'outside' }));

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });
});
