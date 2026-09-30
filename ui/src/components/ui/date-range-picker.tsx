import { Calendar } from 'lucide-react';
import React, { useRef, useState, useEffect } from 'react';
import { cn } from '../../lib/utils';
import dayjs from '../../lib/dayjs';
import { Input } from './input';
import { I18nProps } from '@/i18n/I18nProps';

const DATE_TIME_FORMAT = 'YYYY-MM-DDTHH:mm';
const DATE_TIME_SECONDS_FORMAT = `${DATE_TIME_FORMAT}:ss`;
const DISPLAY_FORMAT = 'YYYY-MM-DD HH:mm:ss';

// Picker values are wall-clock fields; browser timezone rules do not apply.
function parseWallTime(value: string): dayjs.Dayjs {
  const withSeconds = value.split(':').length < 3 ? `${value}:00` : value;
  return dayjs.utc(withSeconds, DATE_TIME_SECONDS_FORMAT, true);
}

interface DateRangePickerProps extends React.HTMLAttributes<HTMLDivElement> {
  fromDate: string | undefined;
  toDate: string | undefined;
  onFromDateChange: (date: string) => void;
  onToDateChange: (date: string) => void;
  fromLabel?: string;
  toLabel?: string;
  onEnterPress?: () => void;
}

// Custom date-time input component
interface CustomDateTimeInputProps {
  value: string | undefined;
  onChange: (value: string) => void;
  id?: string;
  className?: string;
  onEnterPress?: () => void;
}

function CustomDateTimeInput({
  value,
  onChange,
  id,
  className,
  onEnterPress,
}: CustomDateTimeInputProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const hiddenDateInputRef = useRef<HTMLInputElement>(null);
  const [displayValue, setDisplayValue] = useState('');
  const [cursorPosition, setCursorPosition] = useState(0);

  // Format date for display
  useEffect(() => {
    if (value) {
      const date = parseWallTime(value);
      if (date.isValid()) {
        setDisplayValue(date.format(DISPLAY_FORMAT));
      }
    } else {
      setDisplayValue('');
    }
  }, [value]);

  // Restore cursor position after value change
  useEffect(() => {
    if (inputRef.current && cursorPosition > 0) {
      inputRef.current.setSelectionRange(cursorPosition, cursorPosition);
    }
  }, [displayValue, cursorPosition]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newValue = e.target.value;
    setDisplayValue(newValue);
    setCursorPosition(e.target.selectionStart || 0);

    // Emptying the field clears the bound. A partially typed date reports
    // nothing, so the bound only changes once it is whole again.
    if (newValue.trim() === '') {
      onChange('');
      return;
    }
    const parsed = dayjs.utc(newValue, DISPLAY_FORMAT, true);
    if (parsed.isValid()) {
      onChange(parsed.format(DATE_TIME_SECONDS_FORMAT));
    }
  };

  const adjustValue = (increment: number) => {
    const pos = inputRef.current?.selectionStart || 0;
    const parsed = parseWallTime(value || dayjs.utc().format(DATE_TIME_FORMAT));
    if (!parsed.isValid()) {
      return;
    }
    const date = parsed.toDate();

    // Determine which segment to adjust based on cursor position
    // Format: YYYY-MM-DD HH:mm:ss
    // Positions: 0-4 (year), 5-7 (month), 8-10 (day), 11-13 (hour), 14-16 (minute), 17-19 (second)

    if (pos <= 4) {
      date.setUTCFullYear(date.getUTCFullYear() + increment);
    } else if (pos <= 7) {
      date.setUTCMonth(date.getUTCMonth() + increment);
    } else if (pos <= 10) {
      date.setUTCDate(date.getUTCDate() + increment);
    } else if (pos <= 13) {
      date.setUTCHours(date.getUTCHours() + increment);
    } else if (pos <= 16) {
      date.setUTCMinutes(date.getUTCMinutes() + increment);
    } else {
      date.setUTCSeconds(date.getUTCSeconds() + increment);
    }

    onChange(dayjs.utc(date).format(DATE_TIME_SECONDS_FORMAT));
    setCursorPosition(pos);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      e.stopPropagation();
      onEnterPress?.();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      e.stopPropagation();
      adjustValue(1);
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      e.stopPropagation();
      adjustValue(-1);
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
      // Allow normal cursor movement
      setTimeout(() => {
        setCursorPosition(inputRef.current?.selectionStart || 0);
      }, 0);
    }
  };

  const handleDatePickerChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    // The native date picker gives us YYYY-MM-DDTHH:mm format, and an empty
    // value when its own clear control is used.
    const next = e.target.value;
    if (!next || parseWallTime(next).isValid()) {
      onChange(next);
    }
  };

  const openDatePicker = () => {
    hiddenDateInputRef.current?.showPicker?.();
    // Fallback for browsers that don't support showPicker
    hiddenDateInputRef.current?.click();
  };

  return (
    <div className="relative flex items-center">
      <I18nProps><Input
        ref={inputRef}
        id={id}
        type="text"
        value={displayValue}
        onChange={handleInputChange}
        onKeyDown={handleKeyDown}
        onClick={() => setCursorPosition(inputRef.current?.selectionStart || 0)}
        placeholder="YYYY-MM-DD HH:mm:ss"
        className={cn(
          className,
          'w-44 font-mono text-foreground placeholder:text-muted-foreground/60 pt-1'
        )}
      /></I18nProps>
      <I18nProps><button
        type="button"
        onClick={openDatePicker}
        className="px-1 hover:bg-accent rounded-sm transition-colors"
        aria-label="Open date picker"
      >
        <Calendar className="h-4 w-4 text-muted-foreground" />
      </button></I18nProps>
      <input
        ref={hiddenDateInputRef}
        type="datetime-local"
        value={value || ''}
        onChange={handleDatePickerChange}
        className="sr-only"
        tabIndex={-1}
        aria-hidden="true"
      />
    </div>
  );
}

export function DateRangePicker({
  fromDate,
  toDate,
  onFromDateChange,
  onToDateChange,
  onEnterPress,
  className,
  ...props
}: DateRangePickerProps) {
  return (
    <div
      className={cn(
        'relative items-center flex rounded-md border border-input bg-card shadow-sm',
        className
      )}
      {...props}
    >
      <div className="flex flex-col sm:flex-row">
        <div>
          <CustomDateTimeInput
            id="fromDate"
            value={fromDate}
            onChange={onFromDateChange}
            onEnterPress={onEnterPress}
            className="border-0 shadow-none focus-visible:ring-0 text-sm py-0.5 h-9 bg-transparent"
          />
        </div>

        {/* Arrow only visible on sm screens and above */}
        <div className="hidden sm:flex items-center px-1 text-muted-foreground justify-center flex pl-3">
          →
        </div>

        <div>
          <CustomDateTimeInput
            id="toDate"
            value={toDate}
            onChange={onToDateChange}
            onEnterPress={onEnterPress}
            className="border-0 shadow-none focus-visible:ring-0 text-sm py-0.5 h-9 bg-transparent"
          />
        </div>
      </div>
    </div>
  );
}
