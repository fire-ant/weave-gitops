export const parameters = {
  actions: { argTypesRegex: "^on[A-Z].*" },
  controls: {
    matchers: {
      color: /(background|color)$/i,
      date: /Date$/,
    },
  },
};

import { MuiThemeProvider } from "@material-ui/core";
import React from "react";
import { ThemeProvider } from "styled-components";
import theme, { GlobalStyle, muiTheme } from "../ui/lib/theme";

export const decorators = [
  (Story) => (
    <MuiThemeProvider theme={muiTheme}>
      <ThemeProvider theme={theme}>
        <GlobalStyle />
        <Story />
      </ThemeProvider>
    </MuiThemeProvider>
  ),
];