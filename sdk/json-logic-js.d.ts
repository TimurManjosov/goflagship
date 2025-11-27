declare module 'json-logic-js' {
  /**
   * Apply JSON Logic rules to data.
   * @param logic - The JSON Logic expression
   * @param data - The data to apply the logic to
   * @returns The result of evaluating the logic
   */
  function apply(logic: unknown, data: unknown): unknown;

  /**
   * Check if a value is a logic expression.
   * @param logic - The value to check
   * @returns True if the value is a logic expression
   */
  function is_logic(logic: unknown): boolean;

  /**
   * Add a custom operation.
   * @param name - The operation name
   * @param code - The operation implementation
   */
  function add_operation(name: string, code: (...args: unknown[]) => unknown): void;

  /**
   * Remove a custom operation.
   * @param name - The operation name
   */
  function rm_operation(name: string): void;

  /**
   * Get all data variables used in a logic expression.
   * @param logic - The JSON Logic expression
   * @returns Array of variable names
   */
  function uses_data(logic: unknown): string[];

  const _default: {
    apply: typeof apply;
    is_logic: typeof is_logic;
    add_operation: typeof add_operation;
    rm_operation: typeof rm_operation;
    uses_data: typeof uses_data;
  };

  export = _default;
}
