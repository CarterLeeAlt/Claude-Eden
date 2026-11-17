import type { ReactElement, ReactNode } from "react";
import { Tooltip as Primitive } from "radix-ui";

export function Tooltip({
  children,
  content,
}: {
  children: ReactElement;
  content: ReactNode;
}) {
  return (
    <Primitive.Provider delayDuration={500}>
      <Primitive.Root>
        <Primitive.Trigger asChild>{children}</Primitive.Trigger>
        <Primitive.Portal>
          <Primitive.Content className="ui-tooltip" sideOffset={4}>
            {content}
          </Primitive.Content>
        </Primitive.Portal>
      </Primitive.Root>
    </Primitive.Provider>
  );
}
