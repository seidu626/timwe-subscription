export function findIndexByProperty(data: string | any[], key: string | number, value: any) {
    for (let i = 0; i < data.length; i++) {
        if (data[i][key] === value) {
            return i;
        }
    }
    return -1;
}


export function IsNull(obj: any): boolean {
    // because Object.keys(new Date()).length === 0;
    // we have to do some additional check
    return obj // 👈 null and undefined check
        && Object.keys(obj).length === 0
        && Object.getPrototypeOf(obj) === Object.prototype;
}

/// https://stackoverflow.com/questions/7837456/how-to-compare-arrays-in-javascript?page=1&tab=scoredesc#tab-top
export function ArrayCompare(source:any[], array: any[]) {
    return JSON.stringify(source) === JSON.stringify(array);
    // if the other array is a falsy value, return
    if (!array)
        return false;

    // compare lengths - can save a lot of time 
    if (source.length != array.length)
        return false;

    for (var i = 0, l=source.length; i < l; i++) {
        // Check if we have nested arrays
        if (source[i] instanceof Array && array[i] instanceof Array) {
            // recurse into the nested arrays
            if (!source[i].equals(array[i]))
                return false;       
        }           
        else if (source[i] != array[i]) { 
            // Warning - two different object instances will never be equal: {x:20} != {x:20}
            return false;   
        }           
    }       
    return true;
}