import * as React from "react";
import styled from "styled-components";
import { useFeatureFlags } from "../hooks/featureflags";
import { Kind } from "../lib/api/core/types.pb";
import { formatURL, objectTypeToRoute } from "../lib/nav";
import {
  Bucket,
  GitRepository,
  HelmRepository,
  OCIRepository,
  Source,
} from "../lib/objects";
import { showInterval } from "../lib/time";
import { convertGitURLToGitProvider, statusSortHelper } from "../lib/utils";
import DataTable, {
  Field,
  filterByStatusCallback,
  filterConfig,
} from "./DataTable";
import KubeStatusIndicator, { computeMessage } from "./KubeStatusIndicator";
import Link from "./Link";
import Timestamp from "./Timestamp";

type Props = {
  className?: string;
  sources?: Source[];
  appName?: string;
};

function SourcesTable({ className, sources }: Props) {
  const { isFlagEnabled } = useFeatureFlags();

  let initialFilterState = {
    ...filterConfig(sources, "type"),
    ...filterConfig(sources, "namespace"),
    ...filterConfig(sources, "status", filterByStatusCallback),
  };

  if (isFlagEnabled("WEAVE_GITOPS_FEATURE_TENANCY")) {
    initialFilterState = {
      ...initialFilterState,
      ...filterConfig(sources, "tenant"),
    };
  }

  if (isFlagEnabled("WEAVE_GITOPS_FEATURE_CLUSTER")) {
    initialFilterState = {
      ...initialFilterState,
      ...filterConfig(sources, "clusterName"),
    };
  }

  const fields: Field[] = [
    {
      label: "Name",
      value: (s: Source) => (
        <Link
          to={formatURL(objectTypeToRoute(Kind[s.type]), {
            name: s?.name,
            namespace: s?.namespace,
            clusterName: s?.clusterName,
          })}
        >
          {s?.name}
        </Link>
      ),
      sortValue: (s: Source) => s.name || "",
      textSearchable: true,
      maxWidth: 600,
    },
    { label: "Kind", value: "type" },
    { label: "Namespace", value: "namespace" },
    ...(isFlagEnabled("WEAVE_GITOPS_FEATURE_TENANCY")
      ? [{ label: "Tenant", value: "tenant" }]
      : []),
    ...(isFlagEnabled("WEAVE_GITOPS_FEATURE_CLUSTER")
      ? [{ label: "Cluster", value: (s: Source) => s.clusterName }]
      : []),
    {
      label: "Status",
      value: (s: Source) => (
        <KubeStatusIndicator
          short
          conditions={s.conditions}
          suspended={s.suspended}
        />
      ),
      sortValue: statusSortHelper,
      defaultSort: true,
    },
    {
      label: "Message",
      value: (s) => computeMessage(s.conditions),
      maxWidth: 600,
    },
    {
      label: "URL",
      value: (s: Source) => {
        let text;
        let url;
        let link = false;
        switch (s.type) {
          case Kind.GitRepository:
            text = (s as GitRepository).url;
            url = convertGitURLToGitProvider((s as GitRepository).url);
            link = true;
            break;
          case Kind.Bucket:
            text = (s as Bucket).endpoint;
            break;
          case Kind.OCIRepository:
            text = (s as OCIRepository).url;
            break;
          case Kind.HelmRepository:
            text = (s as HelmRepository).url;
            url = text;
            link = true;
            break;
          default:
            text = "-";
            break;
        }
        return link ? (
          <Link newTab href={url}>
            {text}
          </Link>
        ) : (
          text
        );
      },
      maxWidth: 600,
    },
    {
      label: "Reference",
      value: (s: Source) => {
        if (s.type === Kind.GitRepository) {
          const repo = s as GitRepository;
          const ref =
            repo?.reference?.branch ||
            repo?.reference?.commit ||
            repo?.reference?.tag ||
            repo?.reference?.semver;
          return ref;
        }
        return "-";
      },
    },
    {
      label: "Interval",
      value: (s: Source) => showInterval(s.interval),
    },
    {
      label: "Last Updated",
      value: (s: Source) =>
        s.lastUpdatedAt ? <Timestamp time={s.lastUpdatedAt} /> : "-",
      sortValue: (s: Source) => s.lastUpdatedAt || "",
    },
  ];

  return (
    <DataTable
      className={className}
      filters={initialFilterState}
      hasCheckboxes
      rows={sources}
      fields={fields}
    />
  );
}

export default styled(SourcesTable).attrs({ className: SourcesTable.name })`
  td:nth-child(5) {
    white-space: pre-wrap;
    overflow-wrap: break-word;
    word-wrap: break-word;
  }
`;
