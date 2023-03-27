import {
  Checkbox,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@material-ui/core";
import _ from "lodash";
import qs from "query-string";
import * as React from "react";
import { useHistory, useLocation } from "react-router-dom";
import styled from "styled-components";
import Button, { IconButton } from "./Button";
import CheckboxActions from "./CheckboxActions";
import ChipGroup from "./ChipGroup";
import FilterDialog, {
  FilterConfig,
  FilterSelections,
  filterSeparator,
  selectionsToFilters,
} from "./FilterDialog";
import Flex from "./Flex";
import Icon, { IconType } from "./Icon";
import { computeReady, ReadyType } from "./KubeStatusIndicator";
import SearchField from "./SearchField";
import Spacer from "./Spacer";
import Text from "./Text";

export type Field = {
  label: string | number;
  labelRenderer?: string | ((k: any) => string | JSX.Element);
  value: string | ((k: any) => string | JSX.Element | null);
  sortValue?: (k: any) => any;
  textSearchable?: boolean;
  minWidth?: number;
  maxWidth?: number;
  /** boolean for field to initially sort against. */
  defaultSort?: boolean;
  /** boolean for field to implement secondary sort against. */
  secondarySort?: boolean;
};

type FilterState = {
  filters: FilterConfig;
  formState: FilterSelections;
  textFilters: string[];
};

/** DataTable Properties  */
export interface Props {
  /** CSS MUI Overrides or other styling. */
  className?: string;
  /** A list of objects with four fields: `label`, which is a string representing the column header, `value`, which can be a string, or a function that extracts the data needed to fill the table cell, and `sortValue`, which customizes your input to the search function */
  fields: Field[];
  /** A list of data that will be iterated through to create the columns described in `fields`. */
  rows?: any[];
  filters?: FilterConfig;
  dialogOpen?: boolean;
  hasCheckboxes?: boolean;
  hideSearchAndFilters?: boolean;
  emptyMessagePlaceholder?: React.ReactNode;
}
//styled components
const EmptyRow = styled(TableRow)<{ colSpan: number }>`
  td {
    text-align: center;
  }
`;

const TableButton = styled(Button)`
  &.MuiButton-root {
    margin: 0;
    text-transform: none;
    letter-spacing: 0;
  }
  &.MuiButton-text {
    min-width: 0px;
    .selected {
      color: ${(props) => props.theme.colors.neutral40};
    }
  }
  &.arrow {
    min-width: 0px;
  }
`;

const TopBar = styled(Flex)`
  max-width: 100%;
`;

const IconFlex = styled(Flex)`
  position: relative;
  padding: 0 ${(props) => props.theme.spacing.small};
`;

//funcs
export const filterByStatusCallback = (v) => {
  if (v.suspended) return ReadyType.Suspended;
  const ready = computeReady(v["conditions"]);
  if (ready === ReadyType.Reconciling) return ReadyType.Reconciling;
  if (ready === ReadyType.Ready) return ReadyType.Ready;
  return ReadyType.NotReady;
};

export function filterConfig(
  rows,
  key: string,
  computeValue?: (k: any) => any
): FilterConfig {
  const config = _.reduce(
    rows,
    (r, v) => {
      const t = computeValue ? computeValue(v) : v[key];
      if (!_.includes(r, t)) {
        r.push(t);
      }

      return r;
    },
    []
  );

  return { [key]: { options: config, transformFunc: computeValue } };
}

export function filterRows<T>(rows: T[], filters: FilterConfig) {
  if (_.keys(filters).length === 0) {
    return rows;
  }

  return _.filter(rows, (row) => {
    let ok = true;

    _.each(filters, (vals, category) => {
      let value;

      if (vals.transformFunc) value = vals.transformFunc(row);
      // strings
      else value = row[category];

      if (!_.includes(vals.options, value)) {
        ok = false;
        return ok;
      }
    });

    return ok;
  });
}

function filterText(
  rows,
  fields: Field[],
  textFilters: FilterState["textFilters"]
) {
  if (textFilters.length === 0) {
    return rows;
  }

  return _.filter(rows, (row) => {
    let matches = false;

    fields.forEach((field) => {
      if (!field.textSearchable) return matches;

      let value;
      if (field.sortValue) {
        value = field.sortValue(row);
      } else {
        value =
          typeof field.value === "function"
            ? field.value(row)
            : row[field.value];
      }

      for (let i = 0; i < textFilters.length; i++) {
        matches = value.includes(textFilters[i]);
        if (!matches) {
          break;
        }
      }
    });

    return matches;
  });
}

export function initialFormState(cfg: FilterConfig, initialSelections?) {
  if (!initialSelections) {
    return {};
  }
  const allFilters = _.reduce(
    cfg,
    (r, vals, k) => {
      _.each(vals.options, (v) => {
        const key = `${k}${filterSeparator}${v}`;
        const selection = _.get(initialSelections, key);
        if (selection) {
          r[key] = selection;
        } else {
          r[key] = false;
        }
      });

      return r;
    },
    {}
  );
  return allFilters;
}

function toPairs(state: FilterState): string[] {
  const result = _.map(state.formState, (val, key) => (val ? key : null));
  const out = _.compact(result);
  return _.concat(out, state.textFilters);
}

export function parseFilterStateFromURL(search: string) {
  const query = qs.parse(search) as any;
  const state = { initialSelections: {}, textFilters: [] };
  if (query.filters) {
    const split = query.filters.split("_");
    const next = {};
    _.each(split, (filterString) => {
      if (filterString) next[filterString] = true;
    });
    state.initialSelections = next;
  }
  if (query.search) {
    state.textFilters = query.search.split("_").filter((item) => item);
  }
  return state;
}

export function filterSelectionsToQueryString(sel: FilterSelections) {
  let url = "";
  _.each(sel, (value, key) => {
    if (value) {
      url += `${key}_`;
    }
  });
  //this is an object with all the different queries as keys
  let query = qs.parse(location.search);
  //if there are any filters, reassign/create filter query key
  if (url) query["filters"] = url;
  //if the update leaves no filters, remove the filter query key from the object
  else if (query["filters"]) query = _.omit(query, "filters");
  //this turns a parsed search into a legit query string
  return qs.stringify(query);
}

export const sortByField = (
  rows: any[],
  reverseSort: boolean,
  sortFields: Field[],
  useSecondarySort?: boolean
) => {
  const orderFields = [sortFields[0]];
  if (useSecondarySort && sortFields.length > 1)
    orderFields.push(sortFields[1]);

  return _.orderBy(
    rows,
    sortFields.map((s) => {
      return s.sortValue || s.value;
    }),
    orderFields.map((_, index) => {
      // Always sort secondary sort values in the ascending order.
      const sortOrders =
        reverseSort && (!useSecondarySort || index != 1) ? "desc" : "asc";

      return sortOrders;
    })
  );
};
//components
type labelProps = {
  fields: Field[];
  fieldIndex: number;
  sortFieldIndex: number;
  reverseSort: boolean;
  setSortFieldIndex: (index: number) => void;
  setReverseSort: (b: boolean) => void;
};

function SortableLabel({
  fields,
  fieldIndex,
  sortFieldIndex,
  reverseSort,
  setSortFieldIndex,
  setReverseSort,
}: labelProps) {
  const field = fields[fieldIndex];
  const sort = fields[sortFieldIndex];

  return (
    <Flex align start>
      <TableButton
        color="inherit"
        variant="text"
        onClick={() => {
          setReverseSort(sortFieldIndex === fieldIndex ? !reverseSort : false);
          setSortFieldIndex(fieldIndex);
        }}
      >
        <h2 className={sort.label === field.label ? "selected" : ""}>
          {field.label}
        </h2>
      </TableButton>
      <Spacer padding="xxs" />
      {sort.label === field.label ? (
        <Icon
          type={IconType.ArrowUpwardIcon}
          size="base"
          className={reverseSort ? "upward" : "downward"}
        />
      ) : (
        <div style={{ width: "16px" }} />
      )}
    </Flex>
  );
}

/** Form DataTable */
function UnstyledDataTable({
  className,
  fields,
  rows,
  filters,
  hasCheckboxes: checkboxes,
  dialogOpen,
  hideSearchAndFilters,
  emptyMessagePlaceholder,
}: Props) {
  //URL info
  const history = useHistory();
  const location = useLocation();
  const search = location.search;
  const state = parseFilterStateFromURL(search);

  const [filterDialogOpen, setFilterDialogOpen] = React.useState(dialogOpen);
  const [filterState, setFilterState] = React.useState<FilterState>({
    filters: selectionsToFilters(state.initialSelections, filters),
    formState: initialFormState(filters, state.initialSelections),
    textFilters: state.textFilters,
  });

  const handleFilterChange = (sel: FilterSelections) => {
    const filterQuery = filterSelectionsToQueryString(sel);
    history.replace({ ...location, search: filterQuery });
  };

  let filtered = filterRows(rows, filterState.filters);
  filtered = filterText(filtered, fields, filterState.textFilters);
  const chips = toPairs(filterState);

  const doChange = (formState) => {
    if (handleFilterChange) {
      handleFilterChange(formState);
    }
  };

  const handleChipRemove = (chips: string[], filterList) => {
    const next = {
      ...filterState,
    };

    _.each(chips, (chip) => {
      next.formState[chip] = false;
    });

    const filters = selectionsToFilters(next.formState, filterList);

    const textFilters = _.filter(
      next.textFilters,
      (f) => !_.includes(chips, f)
    );

    let query = qs.parse(search);

    if (textFilters.length) query["search"] = textFilters.join("_") + "_";
    else if (query["search"]) query = _.omit(query, "search");
    history.replace({ ...location, search: qs.stringify(query) });

    doChange(next.formState);
    setFilterState({ formState: next.formState, filters, textFilters });
  };

  const handleTextSearchSubmit = (val: string) => {
    if (!val) return;
    const query = qs.parse(search);
    if (!query["search"]) query["search"] = `${val}_`;
    if (!query["search"].includes(val)) query["search"] += `${val}_`;
    history.replace({ ...location, search: qs.stringify(query) });
    setFilterState({
      ...filterState,
      textFilters: _.uniq([...filterState.textFilters, val]),
    });
  };

  const handleClearAll = () => {
    const resetFormState = initialFormState(filters);
    setFilterState({
      filters: {},
      formState: resetFormState,
      textFilters: [],
    });
    const url = qs.parse(location.search);
    //keeps things like clusterName and namespace for details pages
    const cleared = _.omit(url, ["filters", "search"]);
    history.replace({ ...location, search: qs.stringify(cleared) });
  };

  const handleFilterSelect = (filters, formState) => {
    doChange(formState);
    setFilterState({ ...filterState, filters, formState });
  };

  const [sortFieldIndex, setSortFieldIndex] = React.useState(() => {
    let sortFieldIndex = fields.findIndex((f) => f.defaultSort);

    if (sortFieldIndex === -1) {
      sortFieldIndex = 0;
    }

    return sortFieldIndex;
  });

  const secondarySortFieldIndex = fields.findIndex((f) => f.secondarySort);

  const [reverseSort, setReverseSort] = React.useState(false);

  let sortFields = [fields[sortFieldIndex]];

  const useSecondarySort =
    secondarySortFieldIndex > -1 && sortFieldIndex != secondarySortFieldIndex;

  if (useSecondarySort) {
    sortFields = sortFields.concat(fields[secondarySortFieldIndex]);
    sortFields = sortFields.concat(
      fields.filter(
        (_, index) =>
          index != sortFieldIndex && index != secondarySortFieldIndex
      )
    );
  } else {
    sortFields = sortFields.concat(
      fields.filter((_, index) => index != sortFieldIndex)
    );
  }

  const sorted = sortByField(
    filtered,
    reverseSort,
    sortFields,
    useSecondarySort
  );

  const [checked, setChecked] = React.useState([]);

  const r = _.map(sorted, (r, i) => {
    return (
      <TableRow key={r.uid || i}>
        {checkboxes && (
          <TableCell style={{ padding: "0px" }}>
            <Checkbox
              checked={_.includes(checked, r.uid)}
              onChange={(e) => {
                if (e.target.checked) setChecked([...checked, r.uid]);
                else setChecked(_.without(checked, r.uid));
              }}
              color="primary"
            />
          </TableCell>
        )}
        {_.map(fields, (f) => {
          const style: React.CSSProperties = {
            ...(f.minWidth && { minWidth: f.minWidth }),
            ...(f.maxWidth && { maxWidth: f.maxWidth }),
          };

          return (
            <TableCell
              style={Object.keys(style).length > 0 ? style : undefined}
              key={f.label}
            >
              <Text>
                {(typeof f.value === "function" ? f.value(r) : r[f.value]) ||
                  "-"}
              </Text>
            </TableCell>
          );
        })}
      </TableRow>
    );
  });
  return (
    <Flex wide tall column className={className}>
      <TopBar wide align end>
        {checkboxes && <CheckboxActions checked={checked} rows={filtered} />}
        {filters && !hideSearchAndFilters && (
          <>
            <ChipGroup
              chips={chips}
              onChipRemove={(chips) => handleChipRemove(chips, filters)}
              onClearAll={handleClearAll}
            />
            <IconFlex align>
              <SearchField onSubmit={handleTextSearchSubmit} />
              <IconButton
                onClick={() => setFilterDialogOpen(!filterDialogOpen)}
                variant={filterDialogOpen ? "contained" : "text"}
                color="inherit"
              >
                <Icon
                  type={IconType.FilterIcon}
                  size="medium"
                  color="neutral30"
                />
              </IconButton>
            </IconFlex>
          </>
        )}
      </TopBar>
      <Flex wide tall>
        <TableContainer>
          <Table aria-label="simple table">
            <TableHead>
              <TableRow>
                {checkboxes && (
                  <TableCell key={"checkboxes"}>
                    <Checkbox
                      checked={filtered.length === checked.length}
                      onChange={(e) =>
                        e.target.checked
                          ? setChecked(filtered.map((r) => r.uid))
                          : setChecked([])
                      }
                      color="primary"
                    />
                  </TableCell>
                )}
                {_.map(fields, (f, index) => (
                  <TableCell key={f.label}>
                    {typeof f.labelRenderer === "function" ? (
                      f.labelRenderer(r)
                    ) : (
                      <SortableLabel
                        fields={fields}
                        fieldIndex={index}
                        sortFieldIndex={sortFieldIndex}
                        reverseSort={reverseSort}
                        setSortFieldIndex={setSortFieldIndex}
                        setReverseSort={(isReverse) =>
                          setReverseSort(isReverse)
                        }
                      />
                    )}
                  </TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {r.length > 0 ? (
                r
              ) : (
                <EmptyRow colSpan={fields.length}>
                  <TableCell colSpan={fields.length}>
                    <Flex center align>
                      <Icon
                        color="neutral20"
                        type={IconType.RemoveCircleIcon}
                        size="base"
                      />
                      <Spacer padding="xxs" />
                      {emptyMessagePlaceholder || (
                        <Text color="neutral30">No data</Text>
                      )}
                    </Flex>
                  </TableCell>
                </EmptyRow>
              )}
            </TableBody>
          </Table>
        </TableContainer>
        {!hideSearchAndFilters && (
          <FilterDialog
            onFilterSelect={handleFilterSelect}
            filterList={filters}
            formState={filterState.formState}
            open={filterDialogOpen}
          />
        )}
      </Flex>
    </Flex>
  );
}
export const DataTable = styled(UnstyledDataTable)`
  width: 100%;
  flex-wrap: nowrap;
  overflow-x: hidden;
  h2 {
    padding: ${(props) => props.theme.spacing.xs};
    font-size: 12px;
    font-weight: 600;
    color: ${(props) => props.theme.colors.neutral30};
    margin: 0px;
    white-space: nowrap;
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .MuiTableRow-root {
    transition: background 0.5s ease-in-out;
  }
  .MuiTableRow-root:not(.MuiTableRow-head):hover {
    background: ${(props) => props.theme.colors.neutral10};
    transition: background 0.5s ease-in-out;
  }
  table {
    margin-top: ${(props) => props.theme.spacing.small};
  }
  th {
    padding: 0;
    background: ${(props) => props.theme.colors.neutralGray};
    border-top-left-radius: 4px;
    border-top-right-radius: 4px;
    .MuiCheckbox-root {
      padding: 4px 9px;
    }
  }
  td {
    //24px matches th + button + h2 padding
    padding-left: ${(props) => props.theme.spacing.base};
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .filter-options-chip {
    background-color: ${(props) => props.theme.colors.primaryLight05};
  }
`;

export default DataTable;
