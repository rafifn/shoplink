import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react'

type FieldProps = {
  id: string
  label: string
  /** The API's message for this field; shown under the input and read out with it. */
  error?: string
} & Omit<InputHTMLAttributes<HTMLInputElement>, 'id'>

/** A labelled input whose error message is tied to it for screen readers. */
export function Field({ id, label, error, ...input }: FieldProps) {
  const errorId = `${id}-error`
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        name={id}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        {...input}
      />
      {error && (
        <p id={errorId} className="field-error">
          {error}
        </p>
      )}
    </div>
  )
}

type TextAreaFieldProps = {
  id: string
  label: string
  error?: string
} & Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'id'>

/** Like Field, for longer text. */
export function TextAreaField({ id, label, error, ...textarea }: TextAreaFieldProps) {
  const errorId = `${id}-error`
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <textarea
        id={id}
        name={id}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        {...textarea}
      />
      {error && (
        <p id={errorId} className="field-error">
          {error}
        </p>
      )}
    </div>
  )
}
