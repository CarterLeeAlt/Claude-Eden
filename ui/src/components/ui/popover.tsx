import type { ReactElement, ReactNode } from "react";
import { Popover as Primitive } from "radix-ui";

export function Popover({
  children,
  content,
  open,
  onOpenChange,
  label,
}: {
  children: ReactElement;
  content: ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  label: string;
}) {
  return (
    <Primitive.Root open={open} onOpenChange={onOpenChange}>
      <Primitive.Trigger asChild>{children}</Primitive.Trigger>
      <Primitive.Portal>
        <Primitive.Content
          className="ui-popover"
          aria-label={label}
          sideOffset={4}
          collisionPadding={8}
        >
          {content}
        </Primitive.Content>
      </Primitive.Portal>
    </Primitive.Root>
  );
}
