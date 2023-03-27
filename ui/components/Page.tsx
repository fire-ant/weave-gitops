import _ from "lodash";
import React from "react";
import styled from "styled-components";
import useCommon from "../hooks/common";
import { MultiRequestError, RequestError } from "../lib/types";
import Alert from "./Alert";
import Flex from "./Flex";
import Footer from "./Footer";
import LoadingPage from "./LoadingPage";
import Spacer from "./Spacer";

export type PageProps = {
  className?: string;
  children?: any;
  loading?: boolean;
  error?: RequestError | RequestError[] | MultiRequestError[];
};

export const Content = styled(Flex)`
  background-color: ${(props) => props.theme.colors.white};
  border-radius: 10px;
  box-sizing: border-box;
  margin: 0 auto;
  min-height: 100%;
  max-width: 100%;
  padding-bottom: ${(props) => props.theme.spacing.medium};
  padding-left: ${(props) => props.theme.spacing.medium};
  padding-right: ${(props) => props.theme.spacing.medium};
  padding-top: ${(props) => props.theme.spacing.medium};
  overflow: hidden;
`;

const Children = styled(Flex)``;

export function Errors({ error }) {
  const arr = _.isArray(error) ? error : [error];
  if (arr[0])
    return (
      <Flex wide column>
        <Spacer padding="xs" />
        {_.map(arr, (e, i) => (
          <Flex key={i} wide start>
            <Alert title="Error" message={e?.message} severity="error" />
          </Flex>
        ))}
        <Spacer padding="xs" />
      </Flex>
    );
  return null;
}

function Page({ children, loading, error, className }: PageProps) {
  const { settings } = useCommon();

  if (loading) {
    return (
      <Content wide tall start column>
        <LoadingPage />
      </Content>
    );
  }

  return (
    <Content wide between column className={className}>
      <Children column wide tall start>
        <Errors error={error} />
        {children}
      </Children>
      {settings.renderFooter && <Footer />}
    </Content>
  );
}

export default styled(Page)`
  .MuiAlert-root {
    width: 100%;
  }
`;
