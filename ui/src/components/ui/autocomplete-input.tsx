// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import * as React from 'react';

interface AutocompleteInputProps {
  value: string;
  onValueChange: (value: string) => void;
  suggestions: string[];
  placeholder?: string;
  'aria-label'?: string;
  className?: string;
  onEnterPress?: () => void;
}

function AutocompleteInput({
  value,
  onValueChange,
  suggestions,
  placeholder,
  'aria-label': ariaLabel,
  className,
  onEnterPress,
}: AutocompleteInputProps) {
  const [isOpen, setIsOpen] = React.useState(false);
  const [highlightedIndex, setHighlightedIndex] = React.useState(-1);
  const containerRef = React.useRef<HTMLDivElement>(null);
  const listboxId = React.useId();

  // Filter suggestions based on the input value and sort alphabetically
  const filteredSuggestions = React.useMemo(() => {
    const unique = [...new Set(suggestions)].filter((item) => item !== '');
    const sortAlphabetically = (a: string, b: string) =>
      a.toLowerCase().localeCompare(b.toLowerCase());

    if (!value.trim()) {
      return unique.sort(sortAlphabetically);
    }

    const searchLower = value.toLowerCase().trim();
    return unique
      .filter((item) => item.toLowerCase().includes(searchLower))
      .sort(sortAlphabetically);
  }, [value, suggestions]);

  // Reset highlighted index when suggestions change
  React.useEffect(() => {
    setHighlightedIndex(-1);
  }, [filteredSuggestions]);

  // Handle click outside to close dropdown
  React.useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const selectSuggestion = (suggestion: string) => {
    onValueChange(suggestion);
    setIsOpen(false);
    setHighlightedIndex(-1);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    switch (e.key) {
      case 'Enter':
        e.preventDefault();
        if (
          isOpen &&
          highlightedIndex >= 0 &&
          highlightedIndex < filteredSuggestions.length
        ) {
          const suggestion = filteredSuggestions[highlightedIndex];
          if (suggestion !== undefined) {
            selectSuggestion(suggestion);
          }
        } else {
          setIsOpen(false);
          onEnterPress?.();
        }
        break;

      case 'Escape':
        setIsOpen(false);
        setHighlightedIndex(-1);
        break;

      case 'ArrowDown':
        if (filteredSuggestions.length === 0) {
          break;
        }
        e.preventDefault();
        setIsOpen(true);
        setHighlightedIndex((prev) =>
          prev < filteredSuggestions.length - 1 ? prev + 1 : 0
        );
        break;

      case 'ArrowUp':
        if (filteredSuggestions.length === 0) {
          break;
        }
        e.preventDefault();
        setIsOpen(true);
        setHighlightedIndex((prev) =>
          prev > 0 ? prev - 1 : filteredSuggestions.length - 1
        );
        break;

      default:
        break;
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onValueChange(e.target.value);
    setIsOpen(true);
  };

  const handleInputFocus = () => {
    setIsOpen(true);
  };

  return (
    <div ref={containerRef} className="relative">
      <Input
        type="text"
        value={value}
        onChange={handleInputChange}
        onKeyDown={handleKeyDown}
        onFocus={handleInputFocus}
        placeholder={placeholder}
        role="combobox"
        aria-label={ariaLabel ?? placeholder}
        aria-expanded={isOpen && filteredSuggestions.length > 0}
        aria-haspopup="listbox"
        aria-autocomplete="list"
        aria-controls={
          isOpen && filteredSuggestions.length > 0 ? listboxId : undefined
        }
        aria-activedescendant={
          isOpen &&
          filteredSuggestions.length > 0 &&
          highlightedIndex >= 0 &&
          highlightedIndex < filteredSuggestions.length
            ? `${listboxId}-option-${highlightedIndex}`
            : undefined
        }
        autoComplete="off"
        className={className}
      />

      {isOpen && filteredSuggestions.length > 0 && (
        <div
          id={listboxId}
          role="listbox"
          className="absolute z-50 mt-1 w-full max-h-[200px] overflow-y-auto rounded-md border border-border bg-popover shadow-md"
        >
          {filteredSuggestions.map((suggestion, index) => (
            <div
              key={suggestion}
              id={`${listboxId}-option-${index}`}
              role="option"
              aria-selected={index === highlightedIndex}
              className={cn(
                'px-3 py-1.5 text-sm cursor-pointer',
                index === highlightedIndex
                  ? 'bg-accent text-accent-foreground'
                  : 'hover:bg-muted'
              )}
              onClick={() => selectSuggestion(suggestion)}
              // Keep the input's focus so selecting does not blur then
              // re-open the dropdown through onFocus.
              onMouseDown={(e) => e.preventDefault()}
              onMouseEnter={() => setHighlightedIndex(index)}
            >
              {suggestion}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export { AutocompleteInput };
