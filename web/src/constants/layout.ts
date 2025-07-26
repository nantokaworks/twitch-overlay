// Layout constants for FAX display
export const LAYOUT = {
  // Dimensions
  FAX_WIDTH: 250,
  FAX_HEIGHT: 450,
  LABEL_HEIGHT: 40,

  // Positions
  FAX_LEFT_MARGIN: 10,      // FAX部の左マージン
  LABEL_LEFT_MARGIN: 0,    // ラベル部の左マージン
  TOP_POSITION: 0,

  // Animation
  PIXELS_PER_FRAME: 2,
  DISPLAY_DURATION: 5000, // 5 seconds
  FADE_DURATION: 500, // 0.5 seconds
  LAG_DURATION: 2000, // 2 seconds
  TRANSITION_DELAY: 50, // 50ms delay for transition switching

  // LED
  LED_WIDTH: 4,
  LED_HEIGHT: 20,
  LED_TOP_MARGIN: 4,
  LED_RIGHT_MARGIN: 12,

  // Font
  FONT_SIZE: 24,

  // Shake animation
  SHAKE_DURATION: '0.2s',

  // Computed values - Getterメソッドを保持
  get TOTAL_HEIGHT(): number {
    return this.FAX_HEIGHT + this.LABEL_HEIGHT;
  },
  get SLIDE_UP_DISTANCE(): number {
    return -this.TOTAL_HEIGHT;
  }
} as const;