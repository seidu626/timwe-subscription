// https://stackoverflow.com/questions/35272533/flattening-deeply-nested-array-of-objects

import { IsNull } from "./utils";

export function flattenArray(array: any[]) {
    var result: any[] = [];
    array.forEach(function (obj: { children: any; }) {
        if (Array.isArray(obj)) {
            if (obj.length > 0) { // skip empty lists
                result = result.concat(flattenArray(obj));
            }
        }
        if (Array.isArray(obj.children)) {
            result = result.concat(flattenArray(obj.children));
        }

        if (!IsNull(obj)) {
            result.push(obj);
        }

    });
    return result;
}
