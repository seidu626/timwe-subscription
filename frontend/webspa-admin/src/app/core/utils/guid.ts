export class Guid {
    static newGuid() {
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
            const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
    }

    static newIntID(length: number = 8) {
      const timestamp = +new Date();
      const ts = timestamp.toString();
      const parts = ts.split( '').reverse();
      let id = '';

      for ( let i = 0; i < length; ++i ) {
       const index = this.getRandomInt( 0, parts.length - 1 );
       id += parts[index];
      }
      // tslint:disable-next-line:radix
      return parseInt(id);
    }

     static getRandomInt( min: number, max: number ) {
      return Math.floor( Math.random() * ( max - min + 1 ) ) + min;
     }
}
