export class ContactUs {

  constructor(values: Object = {}) {
    Object.assign(this, values);
  }
  public id: number;
  public firstname: string;
  public lastname: string;
  public email: string;
  public phone: string;
  public message: string;
}
