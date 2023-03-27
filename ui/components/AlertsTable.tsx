import qs from "query-string";
import * as React from "react";
import styled from "styled-components";
import { AppContext } from "../contexts/AppContext";
import { useFeatureFlags } from "../hooks/featureflags";
import { Kind } from "../lib/api/core/types.pb";
import { Alert, CrossNamespaceObjectRef } from "../lib/objects";
import { V2Routes } from "../lib/types";
import { statusSortHelper } from "../lib/utils";
import DataTable, {
  Field,
  filterByStatusCallback,
  filterConfig,
} from "./DataTable";
import { filterSeparator } from "./FilterDialog";
import KubeStatusIndicator from "./KubeStatusIndicator";
import Link from "./Link";
import Text from "./Text";
type Props = {
  className?: string;
  rows?: Alert[];
};
export const makeEventSourceLink = (obj: CrossNamespaceObjectRef) => {
  const url =
    obj.kind === Kind.Kustomization || obj.kind === Kind.HelmRelease
      ? V2Routes.Automations
      : V2Routes.Sources;
  let filters = `type${filterSeparator}${obj.kind}_`;
  if (obj.name !== "*") filters += `name${filterSeparator}${obj.name}_`;
  if (obj.namespace !== "*")
    filters += `namespace${filterSeparator}${obj.namespace}_`;
  return url + `?${qs.stringify({ filters: filters })}`;
};

function AlertsTable({ className, rows = [] }: Props) {
  const { isFlagEnabled } = useFeatureFlags();

  let initialFilterState = {
    ...filterConfig(rows, "name"),
    ...filterConfig(rows, "namespace"),
    ...filterConfig(rows, "severity"),
    ...filterConfig(rows, "status", filterByStatusCallback),
  };
  if (isFlagEnabled("WEAVE_GITOPS_FEATURE_CLUSTER")) {
    initialFilterState = {
      ...initialFilterState,
      ...filterConfig(rows, "clusterName"),
    };
  }

  const { setDetailModal } = React.useContext(AppContext);

  const alertFields: Field[] = [
    {
      label: "Name",
      value: (a: Alert) => (
        <Text
          onClick={() =>
            setDetailModal({
              object: a,
            })
          }
          color="primary10"
          pointer
        >
          {a.name}
        </Text>
      ),
      textSearchable: true,
    },
    {
      label: "Namespace",
      value: "namespace",
    },
    {
      label: "Severity",
      value: "severity",
    },
    {
      label: "Event Sources",
      value: (a) => {
        return (
          <ul className="event-sources">
            {a?.eventSources?.map((obj: CrossNamespaceObjectRef, index) => {
              return obj.name && obj.namespace && obj.kind ? (
                <Link
                  className="event-sources"
                  key={index}
                  to={makeEventSourceLink(obj)}
                >
                  {obj.kind}: {obj.namespace}/{obj.name}
                </Link>
              ) : (
                <li className="event-sources" key={index}>
                  {obj.kind}: {obj.namespace}/{obj.name}
                </li>
              );
            })}
          </ul>
        );
      },
      labelRenderer: () => <h2 className="event-sources">Event Sources</h2>,
    },
    {
      label: "Status",
      value: (a: Alert) =>
        a.conditions.length > 0 ? (
          <KubeStatusIndicator
            short
            conditions={a.conditions}
            suspended={a.suspended}
          />
        ) : null,
      sortValue: statusSortHelper,
    },
    ...(isFlagEnabled("WEAVE_GITOPS_FEATURE_CLUSTER")
      ? [{ label: "Cluster", value: (obj) => obj.clusterName }]
      : []),
  ];

  return (
    <DataTable
      className={className}
      fields={alertFields}
      rows={rows}
      filters={initialFilterState}
    />
  );
}

export default styled(AlertsTable).attrs({ className: AlertsTable.name })`
  //these styles did not apply when wrapped in .event-sources, only in this more repetitive format
  ul {
    &.event-sources {
      padding: 0px;
    }
  }
  h2 {
    &.event-sources {
      padding-left: ${(props) => props.theme.spacing.small};
    }
  }
  ${Link}, li {
    &.event-sources {
      display: block;
    }
  }
`;
