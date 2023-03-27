import * as React from "react";
import styled from "styled-components";
import CopyToClipboard from "./CopyToCliboard";
import DataTable, { Field } from "./DataTable";
import Flex from "./Flex";
import MessageBox from "./MessageBox";
import Text from "./Text";

type Props = {
  className?: string;
  rows: string[];
};

function UserGroupsTable({ className, rows }: Props) {
  const providerFields: Field[] = [
    {
      label: "Group Name",
      sortValue: (v) => v,
      value: (item) => (
        <div className="GroupContainer">
          <p className="GroupText">{item}</p>
          <CopyToClipboard value={item}></CopyToClipboard>
        </div>
      ),
    },
  ];

  if (!rows?.length)
    return (
      <Flex wide tall column align>
        <MessageBox>
          <Text size="large" semiBold>
            You are not subscribed to any group
          </Text>
        </MessageBox>
      </Flex>
    );

  return (
    <DataTable className={className} rows={rows} fields={providerFields} />
  );
}

export default styled(UserGroupsTable).attrs({
  className: UserGroupsTable.name,
})`
  .GroupText {
    margin-right: 8px;
  }
  .GroupContainer {
    display: flex;
    justify-content: start;
    align-items: center;
  }
`;
