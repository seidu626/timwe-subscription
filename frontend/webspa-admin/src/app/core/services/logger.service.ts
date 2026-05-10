import {ErrorHandler, Injectable} from '@angular/core';
import {environment} from '../../../environments/environment';

@Injectable({
  providedIn: 'root',
})
export class LoggerService {
  constructor() {}

  log(message?: any, ...optionalParams: any[]) {
    if (!environment.production) {
      const callerInfo = this.getCallerInfo();
      console.log(callerInfo, message, ...optionalParams);
    }
  }

  error(message?: any, ...optionalParams: any[]) {
    if (!environment.production) {
      const callerInfo = this.getCallerInfo();
      console.error(callerInfo, message, ...optionalParams);
    }
  }


  /**
   * This method does not display anything unless used in the inspector.
   *  Prints to `stdout` the array `array` formatted as a table.
   */
  table(tabularData: any, properties?: Array<string>) {
    if (!environment.production) {
      const callerInfo = this.getCallerInfo();
      console.table(tabularData, properties);
    }
  }


  /**
   * The {@link console.warn} function is an alias for {@link console.error}.
   */
  warn(message?: any, ...optionalParams: any[]) {
    if (!environment.production) {
      const callerInfo = this.getCallerInfo();
      // console.log(`${callerInfo}: ${value}`, ...rest);
      console.warn(callerInfo, message, ...optionalParams);
    }
  }

  /**
   * The `console.debug()` function is an alias for {@link console.log}.
   */
  debug(message?: any, ...optionalParams: any[]) {
    if (!environment.production) {
      const callerInfo = this.getCallerInfo();
      // console.log(`${callerInfo}: ${value}`, ...rest);
      console.debug(message, ...optionalParams);
    }
  }

  private getCallerInfo(): string {
    try {
      throw new Error();
    } catch (e: any) {
      const stackLines = e.stack.split('\n');
      if (stackLines.length >= 4) {
        const callerLine = stackLines[3].trim();
        return callerLine;
      }
    }
    return 'Unknown';
  }
}


@Injectable({
  providedIn: 'root',
})
export class __LoggerService {

  constructor(private errorHandler: ErrorHandler) {
  }



  clear(): void;
  clear(): void;
  clear(): void {
    throw new Error('Method not implemented.');
  }

  count(label?: string | undefined): void;
  count(label?: string | undefined): void;
  count(label?: unknown): void {
    throw new Error('Method not implemented.');
  }

  countReset(label?: string | undefined): void;
  countReset(label?: string | undefined): void;
  countReset(label?: unknown): void {
    throw new Error('Method not implemented.');
  }

  dir(item?: any, options?: any): void;
  dir(obj?: unknown, options?: unknown): void {
    throw new Error('Method not implemented.');
  }

  dirxml(...data: any[]): void;
  dirxml(...data: any[]): void;
  dirxml(...data: unknown[]): void {
    throw new Error('Method not implemented.');
  }

  group(...data: any[]): void;
  group(...label: any[]): void;
  group(...label: unknown[]): void {
    throw new Error('Method not implemented.');
  }

  groupCollapsed(...data: any[]): void;
  groupCollapsed(...label: any[]): void;
  groupCollapsed(...label: unknown[]): void {
    throw new Error('Method not implemented.');
  }

  groupEnd(): void;
  groupEnd(): void;
  groupEnd(): void {
    throw new Error('Method not implemented.');
  }

  info(...data: any[]): void;
  info(message?: any, ...optionalParams: any[]): void;
  info(message?: unknown, ...optionalParams: unknown[]): void {
    throw new Error('Method not implemented.');
  }

  time(label?: string | undefined): void;
  time(label?: string | undefined): void;
  time(label?: unknown): void {
    throw new Error('Method not implemented.');
  }

  timeEnd(label?: string | undefined): void;
  timeEnd(label?: string | undefined): void;
  timeEnd(label?: unknown): void {
    throw new Error('Method not implemented.');
  }

  timeLog(label?: string | undefined, ...data: any[]): void;
  timeLog(label?: string | undefined, ...data: any[]): void;
  timeLog(label?: unknown, ...data: unknown[]): void {
    throw new Error('Method not implemented.');
  }

  timeStamp(label?: string | undefined): void;
  timeStamp(label?: string | undefined): void;
  timeStamp(label?: unknown): void {
    throw new Error('Method not implemented.');
  }

  trace(...data: any[]): void;
  trace(message?: any, ...optionalParams: any[]): void;
  trace(message?: unknown, ...optionalParams: unknown[]): void {
    throw new Error('Method not implemented.');
  }



  markTimeline(label?: string | undefined): void {
    throw new Error('Method not implemented.');
  }

  profile(label?: string | undefined): void {
    throw new Error('Method not implemented.');
  }

  profileEnd(label?: string | undefined): void {
    throw new Error('Method not implemented.');
  }

  timeline(label?: string | undefined): void {
    throw new Error('Method not implemented.');
  }

  timelineEnd(label?: string | undefined): void {
    throw new Error('Method not implemented.');
  }

  /**
   * Prints to `stdout` with newline.
   */
  log(message?: any, ...optionalParams: any[]) {
    if (!environment.production) {
      const callerInfo = this.getCallerInfo();
      console.log(message, ...optionalParams);
    }
  }


  /**
   * Prints to `stderr` with newline.
   */
  error(message?: any, ...optionalParams: any[]) {
    if (!environment.production) {
      const callerInfo = this.getCallerInfo();
      // console.error(`${callerInfo} |  'message: ' + ${message}, 'stack: ' + ${stack} `);
      console.error(callerInfo, message, ...optionalParams);
    }
  }

  /**
   * The {@link console.warn} function is an alias for {@link ErrorHandler}.
   */
  handleError(error: Error) {
    this.errorHandler.handleError(error);
  }


  /**
   * A simple assertion test that verifies whether `value` is truthy.
   * If it is not, an `AssertionError` is thrown.
   * If provided, the error `message` is formatted using `utils.format()` and used as the error message.
   */
  assert(value: any, message?: string, ...optionalParams: any[]): void {
    if (!environment.production) {
      const callerInfo = this.getCallerInfo();
      // console.log(`${callerInfo}: ${value}`, ...rest);
      console.assert(value, message, ...optionalParams);
    }
  }

  public getCallerInfoV1(): string {
    try {
      const stack = new Error().stack;
      if (stack) {
        const lines = stack.split('\n');
        // Lines[2] contains file and line number info
        const callerLine = lines[2].trim();
        return callerLine;
      }
    } catch (error) {
    }
    return 'Unknown';
  }

  private getCallerInfo(): string {
    try {
      throw new Error();
    } catch (e: any) {
      // The stack property of the Error object contains the call stack
      const stackLines = e.stack.split('\n');
      // The 3rd line usually contains the caller information
      if (stackLines.length >= 4) {
        const callerLine = stackLines[3].trim();
        return callerLine;
      }
    }
    return 'Unknown';
  }

  private getCallerInfoV3(): string {
    const stackTrace = this.captureStackTrace();
    if (stackTrace) {
      // Extract the file name and line number from the stack trace
      const callerLineMatch = stackTrace.match(/at\s+.*\/(.*):(\d+:\d+)/);
      if (callerLineMatch) {
        const [_, fileName, lineNumber] = callerLineMatch;
        return `${fileName}:${lineNumber}`;
      }
    }
    return 'Unknown';
  }

  private captureStackTrace(): string | undefined {
    try {
      throw new Error();
    } catch (e: any) {
      return e.stack;
    }
  }

}
