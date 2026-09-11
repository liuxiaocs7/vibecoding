import React from 'react';
import { ChevronDown } from 'lucide-react';

type ThemedSelectProps = React.SelectHTMLAttributes<HTMLSelectElement> & {
  chevronClassName?: string;
  isLight?: boolean;
};

export const ThemedSelect: React.FC<ThemedSelectProps> = ({
  className = '',
  chevronClassName = '',
  isLight = false,
  children,
  ...props
}) => (
  <div className="relative">
    <select
      {...props}
      data-color-scheme={isLight ? 'light' : 'dark'}
      className={`vc-control w-full pr-9 cursor-pointer ${className}`}
    >
      {children}
    </select>
    <ChevronDown
      className={`pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 opacity-60 ${chevronClassName}`}
    />
  </div>
);
