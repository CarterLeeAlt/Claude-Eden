import {
  forwardRef,
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type TextareaHTMLAttributes,
} from "react";

export const Button = forwardRef<
  HTMLButtonElement,
  ButtonHTMLAttributes<HTMLButtonElement> & {
    variant?: "primary" | "secondary" | "quiet";
    compact?: boolean;
  }
>(function Button(
  {
    className = "",
    variant = "secondary",
    compact = false,
    type = "button",
    ...props
  },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      className={`ui-button${variant === "primary" ? " primary" : variant === "quiet" ? " quiet-button" : ""}${compact ? " ui-control--compact" : ""} ${className}`}
      {...props}
    />
  );
});
export const Input = forwardRef<
  HTMLInputElement,
  InputHTMLAttributes<HTMLInputElement>
>(function Input({ className = "", ...props }, ref) {
  return <input ref={ref} className={`ui-input ${className}`} {...props} />;
});
export const Textarea = forwardRef<
  HTMLTextAreaElement,
  TextareaHTMLAttributes<HTMLTextAreaElement>
>(function Textarea({ className = "", ...props }, ref) {
  return (
    <textarea
      ref={ref}
      className={`ui-input ui-textarea ${className}`}
      {...props}
    />
  );
});
export const Checkbox = forwardRef<
  HTMLInputElement,
  Omit<InputHTMLAttributes<HTMLInputElement>, "type">
>(function Checkbox(props, ref) {
  return <Input ref={ref} type="checkbox" {...props} />;
});
export const Switch = forwardRef<
  HTMLInputElement,
  Omit<InputHTMLAttributes<HTMLInputElement>, "type" | "role">
>(function Switch(props, ref) {
  return <Input ref={ref} type="checkbox" role="switch" {...props} />;
});
