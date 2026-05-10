import { Observable, defer, isObservable, of } from 'rxjs';
import { shareReplay, first, mergeMap } from 'rxjs/operators';

// https://itnext.io/the-magic-of-rxjs-sharing-operators-and-their-differences-3a03d699d255
// https://stackblitz.com/edit/pjlamb12-rxjs-caching-and-refreshing-data?file=src%2Fapp%2Frenew-after-timer.observable.ts
let returnObs$: Observable<any>;
const createReturnObs = (obs: Observable<any>, time: number, bufferReplays: number) =>
	(returnObs$ = obs.pipe(shareReplay(bufferReplays, time)));

export function renewAfterTimer(obs: Observable<any>, time: number, bufferReplays: number = 1) {
	return createReturnObs(obs, time, bufferReplays).pipe(
		first(null, defer(() => createReturnObs(obs, time, bufferReplays))),
		mergeMap(d => (isObservable(d) ? d : of(d))),
	);
}
