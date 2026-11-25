import type { InputHTMLAttributes } from 'react'

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
